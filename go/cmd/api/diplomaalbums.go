package main

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jrgensen/cqrs"

	"nathejk.dk/internal/blob"
	"nathejk.dk/nathejk/table/album"
	"nathejk.dk/nathejk/table/photo"
)

// The patrol photographs, filed into albums by type.
//
// # What it does
//
// foto files each patrol photograph with a camera-app type: `start` at the start line, `maal` at the finish. Each
// type has one album — "Start" and "Slut" — holding **one photograph per patrol: the newest of that type**. A
// patrol photographed again replaces its earlier photograph in the album. Each photograph is put in the library,
// tagged with the patrol, and named after the patrol's number zero-padded to three digits (`007.jpg`), and the
// album is created sorted by filename ascending — so it reads in patrol order.
//
// This replaced a curator button ("Læg diplombilleder i album", task 397), which filed the diploma cover into one
// album on demand. Nobody has to press anything now.
//
// # When it runs
//
//   - **Once at boot, after the projections it reads have caught up.** That is what the button used to cover:
//     photographs taken, purged or refused while the app was down. Before catch-up the projections describe the
//     past, and filing from them would add photographs that have since been replaced.
//   - **After every live `photographed`, `photopurged` and `photoconsented`**, debounced: a camera uploading a
//     dozen patrols in a minute causes one sync, not a dozen.
//
// Each run is the whole year, not just the patrol in the message. It is idempotent — everything is keyed by
// content and compared with what is already there — so a run with nothing to do publishes nothing, and one code
// path serves both moments.
//
// # Consent
//
// A refusing patrol is not returned by `patrolphoto.Latest`, so its photographs are never filed, and a membership
// it already has is removed. Each run also first sweeps every refusing patrol out of every album, catching a
// refusal that arrived while the app was down (see photoconsent.go).
//
// # What a curator can still do
//
// Delete a photograph from the library: it stays deleted, and the patrol is left out of the album rather than
// re-filed. Rename, describe, publish and re-sort the albums. What the albums do **not** keep is a hand removal of
// a patrol's current photograph — these albums are managed, and the next run puts it back.
//
// # No re-encode
//
// The bytes are foto's display rendition, which foto rebuilt from pixels and which therefore carries no EXIF (see
// patrolphoto/table.sql). They are fetched and hash-verified by `internal/photobytes` into the same blob store the
// library reads, so the library id is the ref, and nothing is stored twice.

// diplomaAlbums maps a camera-app type to the album it fills.
var diplomaAlbums = []struct{ Type, Title, Description string }{
	{"start", "Start", "Patruljerne ved starten, ét billede pr. patrulje."},
	{"maal", "Slut", "Patruljerne ved målet, ét billede pr. patrulje."},
}

const (
	// diplomaSyncTimeout bounds one run. A year is a few hundred patrols per type, most of whose photographs are
	// already in the store from rendering diplomas; the rest are one fetch from foto each.
	diplomaSyncTimeout = 5 * time.Minute
	// diplomaSyncDebounce is how long a live event waits for more before the run starts. Long enough to gather a
	// camera's burst of uploads, and to let the projections fold the event that triggered it.
	diplomaSyncDebounce = 5 * time.Second
	// diplomaCatchupFallback is how long the boot run waits for the projections to report catching up before it
	// runs anyway. A production boot on a clean database filed nothing and logged nothing, and the wait was the one
	// step that could stall without a trace — so it may no longer stall at all. Running early costs at most a run
	// that files part of the year; the next event or boot completes it.
	diplomaCatchupFallback = 3 * time.Minute
)

// diplomaReactor runs the sync at boot and after the events that change which photograph a patrol has.
type diplomaReactor struct {
	// app is set once the application exists; the broker may connect before it does.
	app     atomic.Pointer[application]
	started time.Time
	year    string
	logger  *slog.Logger

	// pending counts the projections the sync reads that have not yet caught up. Zero means the boot run can go.
	pending atomic.Int32
	// waiting names them, for the log: which projection a stalled boot is waiting for is the whole diagnosis.
	waitMu  sync.Mutex
	waiting map[string]bool
	// forced is set when the fallback ran the boot sync without the catch-up; live events are then served too.
	forced atomic.Bool
	// armed is set once every awaited projection has been registered (`arm`).
	//
	// **Without it, pending == 0 is ambiguous**, and that ambiguity emptied the Start album in production. The broker
	// connects in the background, so `setApp` routinely runs *before* any projection is awaited: pending is 0
	// because nothing has registered yet, not because everything caught up. The boot run then fired against
	// projections that had not replayed — harmless on a warm database, where the tables still hold last boot's rows,
	// and a run with nothing to file on a clean one. The real catch-up then found the boot run already spent.
	armed atomic.Bool
	// bootRuns counts boot runs started: one, ever. Read by the tests.
	bootRuns atomic.Int32
	booted   sync.Once

	// mu serialises runs and guards the debounce timers.
	mu     sync.Mutex
	run    sync.Mutex
	timers map[string]*time.Timer
}

func newDiplomaReactor(year string, logger *slog.Logger) *diplomaReactor {
	return &diplomaReactor{started: time.Now(), year: year, logger: logger, timers: map[string]*time.Timer{},
		waiting: map[string]bool{}}
}

func (d *diplomaReactor) Consumes() []cqrs.Subject {
	return []cqrs.Subject{
		cqrs.SubjectFromStr("NATHEJK:*.patrulje.*.photographed"),
		cqrs.SubjectFromStr("NATHEJK:*.patrulje.*.photopurged"),
		cqrs.SubjectFromStr("NATHEJK:*.patrulje.*.photoconsented"),
	}
}

// HandleMessage schedules a run for the message's year. Replayed messages are ignored: the boot run covers them.
func (d *diplomaReactor) HandleMessage(msg cqrs.Message) error {
	if msg.Time().Before(d.started) {
		return nil
	}
	parts := msg.Subject().Parts()
	if len(parts) < 2 || parts[1] == "" {
		return fmt.Errorf("diploma albums: unexpected subject %q", msg.Subject().Subject())
	}
	d.schedule(parts[1])
	return nil
}

var _ cqrs.Consumer = (*diplomaReactor)(nil)

// awaitCatchup wraps a projection the sync reads, so the boot run waits for it to have replayed.
//
// The stream library reports catch-up per handler (`stream.CatchupListener`), and the projections themselves do not
// listen for it, so the wrapper does.
func (d *diplomaReactor) awaitCatchup(name string, c cqrs.Consumer) cqrs.Consumer {
	d.pending.Add(1)
	d.waitMu.Lock()
	d.waiting[name] = true
	d.waitMu.Unlock()
	return &catchupSignal{Consumer: c, done: func() {
		d.waitMu.Lock()
		delete(d.waiting, name)
		d.waitMu.Unlock()
		left := d.pending.Add(-1)
		d.logger.Info("diploma albums: a projection has caught up", "projection", name, "stillWaitingFor", left)
		if left == 0 {
			d.ready()
		}
	}}
}

// stillWaiting lists the projections that have not reported catching up.
func (d *diplomaReactor) stillWaiting() []string {
	d.waitMu.Lock()
	defer d.waitMu.Unlock()
	out := make([]string, 0, len(d.waiting))
	for name := range d.waiting {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// arm records that every projection the boot run waits for has been registered. Called once, from the broker's
// connect callback, after the awaited projections are wrapped and before they are subscribed.
func (d *diplomaReactor) arm() {
	d.armed.Store(true)
	d.logger.Info("diploma albums: the boot run waits for the projections to catch up", "waitingFor", d.stillWaiting())
	time.AfterFunc(diplomaCatchupFallback, d.fallback)
	if d.pending.Load() == 0 {
		d.ready()
	}
}

// setApp hands the reactor the application, which may arrive before or after the projections catch up.
func (d *diplomaReactor) setApp(app *application) {
	d.app.Store(app)
	d.ready()
}

// fallback runs the boot sync if the catch-up has still not arrived, and says loudly which projection never
// reported. See diplomaCatchupFallback.
func (d *diplomaReactor) fallback() {
	if d.pending.Load() == 0 {
		return
	}
	if d.app.Load() == nil {
		// No application yet, so nothing could run; setApp will call ready, and this tries again.
		time.AfterFunc(diplomaCatchupFallback, d.fallback)
		return
	}
	d.logger.Warn("diploma albums: the projections did not report catching up; running the boot sync anyway",
		"after", diplomaCatchupFallback, "stillWaitingFor", d.stillWaiting())
	d.forced.Store(true)
	d.booted.Do(func() {
		d.bootRuns.Add(1)
		go d.sync(d.year)
	})
}

// ready starts the boot run once the application is there, the projections are registered, and all have caught up.
// Called from each of those three events; only the last one to happen gets past the checks.
func (d *diplomaReactor) ready() {
	if d.app.Load() == nil || !d.armed.Load() || d.pending.Load() > 0 {
		return
	}
	d.booted.Do(func() {
		d.logger.Info("diploma albums: the projections have caught up; running the boot sync")
		d.bootRuns.Add(1)
		go d.sync(d.year)
	})
}

// schedule runs the sync for a year once the events stop arriving for diplomaSyncDebounce.
func (d *diplomaReactor) schedule(year string) {
	if d.app.Load() == nil || ((!d.armed.Load() || d.pending.Load() > 0) && !d.forced.Load()) {
		// Still booting; the boot run will see this event's effect.
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if t, ok := d.timers[year]; ok {
		t.Reset(diplomaSyncDebounce)
		return
	}
	d.timers[year] = time.AfterFunc(diplomaSyncDebounce, func() {
		d.mu.Lock()
		delete(d.timers, year)
		d.mu.Unlock()
		d.sync(year)
	})
}

func (d *diplomaReactor) sync(year string) {
	app := d.app.Load()
	if app == nil || year == "" {
		return
	}
	d.run.Lock()
	defer d.run.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), diplomaSyncTimeout)
	defer cancel()
	res, err := app.syncDiplomaAlbums(ctx, year)
	if err != nil {
		d.logger.Error("filing the patrol photographs into albums", "year", year, "err", err,
			"uploaded", res.Uploaded, "added", res.Added, "replaced", res.Replaced)
		return
	}
	// **Every run logs**, including one with nothing to do. It used to log only on a change or a skip, and that is
	// how a production boot that filed nothing left no trace of whether it had run at all. `patrols` is what each
	// type's read returned: zero there with photographs on the stream is its own diagnosis.
	d.logger.Info("filed the patrol photographs into albums", "year", year,
		"patrols", res.Patrols, "changed", res.changed(),
		"uploaded", res.Uploaded, "renamed", res.Renamed, "tagged", res.Tagged, "added", res.Added,
		"replaced", res.Replaced, "refusedRemoved", res.RefusedRemoved, "resorted", res.Resorted,
		"skipped", res.Skipped)
}

// catchupSignal is a projection that also reports when it has replayed.
type catchupSignal struct {
	cqrs.Consumer
	once sync.Once
	done func()
}

func (c *catchupSignal) CaughtUp() { c.once.Do(c.done) }

// diplomaSyncResult is what one run did, for the log.
type diplomaSyncResult struct {
	Uploaded int
	Renamed  int
	Tagged   int
	Added    int
	// Replaced are memberships removed because the patrol has a newer photograph, or none any more.
	Replaced int
	// RefusedRemoved are memberships taken down, in any album, because the patrol refused photographs.
	RefusedRemoved int
	Resorted       int
	// Skipped are patrols whose photograph could not be fetched, or was deleted from the library.
	Skipped int
	// Patrols is how many patrols each type's read returned ("start", "maal"), for the log.
	Patrols map[string]int
}

func (r diplomaSyncResult) changed() bool {
	return r.Uploaded+r.Renamed+r.Tagged+r.Added+r.Replaced+r.RefusedRemoved+r.Resorted > 0
}

// syncDiplomaAlbums brings both albums in line with the patrols' newest photographs. The result counts what was
// done even when it returns an error.
func (app *application) syncDiplomaAlbums(ctx context.Context, year string) (diplomaSyncResult, error) {
	var res diplomaSyncResult
	if app.models.AlbumCurator == nil || app.models.PhotoCurator == nil ||
		app.models.PatrolPhotos == nil || app.photos == nil {
		return res, fmt.Errorf("the albums or the photo library are unavailable")
	}

	// Refusals first, across every album: one that arrived while the app was down was never reacted to (see
	// photoconsent.go).
	refused, err := app.models.PatrolPhotos.Refused(year)
	if err != nil {
		return res, err
	}
	for _, teamID := range refused {
		n, err := app.removeRefusedFromAlbums(year, teamID)
		res.RefusedRemoved += n
		if err != nil {
			return res, err
		}
	}

	for _, a := range diplomaAlbums {
		if err := app.syncDiplomaAlbum(ctx, year, a.Type, a.Title, a.Description, &res); err != nil {
			return res, fmt.Errorf("album %q: %w", a.Title, err)
		}
	}
	return res, nil
}

// syncDiplomaAlbum files one type's photographs into its album.
func (app *application) syncDiplomaAlbum(ctx context.Context, year, typ, title, description string, res *diplomaSyncResult) error {
	latest, err := app.models.PatrolPhotos.Latest(year, typ)
	if err != nil {
		return err
	}
	if res.Patrols == nil {
		res.Patrols = map[string]int{}
	}
	res.Patrols[typ] = len(latest)
	if len(latest) == 0 {
		// Nothing of this type yet; no empty album is created for it.
		return nil
	}

	albumID, err := app.diplomaAlbumID(year, title, description)
	if err != nil {
		return err
	}
	header, items, found, err := app.models.AlbumCurator.Album(year, albumID)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("the album %s is not in the projection yet", albumID)
	}
	live := make(map[string]bool, len(items))
	for _, it := range items {
		if !it.Removed {
			live[it.PhotoID] = true
		}
	}
	next, err := app.models.AlbumCurator.NextOrdinal(year, albumID)
	if err != nil {
		return err
	}

	// The photograph each patrol should have in this album, and its filename.
	want := make(map[string]string, len(latest)) // teamId → photoId
	names := map[string]string{}                 // photoId → filename, for the sort below
	var added []string
	for _, t := range latest {
		if err := ctx.Err(); err != nil {
			return err
		}
		photoID := t.Ref
		fileName := diplomaFileName(t.Number, t.ContentType)

		existing, known, err := app.models.PhotoCurator.Photo(year, photoID)
		if err != nil {
			return err
		}
		// **No videos in Start and Slut** (PRD 029 §11 Q7). A video may be patrol-tagged like a photograph, so a
		// refusal can find it, but these two albums are one photograph per patrol and a clip is not one.
		if strings.HasPrefix(t.ContentType, "video/") || (known && existing.Kind == "video") {
			res.Skipped++
			continue
		}
		if known && existing.Deleted {
			// A curator took it down, possibly because somebody asked. Not this sync's decision to reverse, and
			// the patrol's older photograph is not a substitute: it was replaced for a reason too.
			res.Skipped++
			continue
		}
		if !known {
			uploaded, err := app.diplomaPhotoUploaded(ctx, year, t.Ref, t.ThumbRef, t.Width, t.Height, fileName)
			if err != nil {
				app.Logger.Info("a patrol photograph could not be fetched for the library",
					"team", t.TeamID, "ref", t.Ref, "err", err)
				res.Skipped++
				continue
			}
			if err := app.publishPhoto(year, photoID, photo.VerbUploaded, uploaded); err != nil {
				return err
			}
			res.Uploaded++
		} else if fileName != "" && existing.FileName != fileName {
			if err := app.publishPhoto(year, photoID, photo.VerbUpdated, photo.Updated{
				PhotoID:   photoID,
				Year:      year,
				FileName:  &fileName,
				UpdatedAt: time.Now().UTC(),
			}); err != nil {
				return err
			}
			res.Renamed++
		}
		want[t.TeamID] = photoID
		if fileName != "" {
			names[photoID] = fileName
		}

		// Tagged always, number or not: the tag is how a refusal finds the photograph again (`TeamAlbumItems`), and
		// how the removal pass below knows whose it is. An untagged photograph of a patrol would stay in every album
		// after the patrol said no.
		tagged, err := app.photoTaggedWith(year, photoID, t.TeamID)
		if err != nil {
			return err
		}
		if !tagged {
			if err := app.publishPhoto(year, photoID, photo.VerbPatrolTagged, photo.PatrolTagged{
				PhotoID:  photoID,
				Year:     year,
				TeamID:   t.TeamID,
				Number:   t.Number,
				TaggedAt: time.Now().UTC(),
			}); err != nil {
				return err
			}
			res.Tagged++
		}

		if !live[photoID] {
			if err := app.publishAlbum(year, album.VerbItemAdded, albumID, album.ItemAdded{
				AlbumID: albumID,
				Year:    year,
				Ordinal: next,
				PhotoID: photoID,
				AddedAt: time.Now().UTC(),
			}); err != nil {
				return err
			}
			live[photoID] = true
			added = append(added, photoID)
			next++
			res.Added++
		}
	}

	// One photograph per patrol: any other photograph of a patrol in this album goes — an earlier one replaced by a
	// newer, or one whose patrol no longer has a photograph of this type (purged, or refused). A photograph tagged
	// with no patrol is a curator's addition and is left alone.
	keep := make(map[string]bool, len(want))
	for _, id := range want {
		keep[id] = true
	}
	removed := map[string]bool{}
	for _, it := range items {
		if it.Removed || keep[it.PhotoID] {
			continue
		}
		tags, err := app.models.PhotoCurator.Tags(year, it.PhotoID)
		if err != nil {
			return err
		}
		// A video goes whatever its tags say: these albums never hold one (PRD 029 §11 Q7), however it got there.
		p, known, err := app.models.PhotoCurator.Photo(year, it.PhotoID)
		if err != nil {
			return err
		}
		isVideo := known && p.Kind == "video"
		// Otherwise untagged is a curator's addition, and stays.
		if len(tags) == 0 && !isVideo {
			continue
		}
		reason := "Erstattet af patruljens nyeste billede"
		if isVideo {
			reason = "Videoer hører ikke til i dette album"
		}
		if err := app.publishAlbum(year, album.VerbItemRemoved, albumID, album.ItemRemoved{
			AlbumID:   albumID,
			Year:      year,
			PhotoID:   it.PhotoID,
			Ordinal:   it.Ordinal,
			Reason:    reason,
			RemovedAt: time.Now().UTC(),
		}); err != nil {
			return err
		}
		removed[it.PhotoID] = true
		res.Replaced++
	}

	// Apply the album's sort mode, with the filenames just given — the projection may not have folded the renames
	// yet, so its keys would sort the album by the old names.
	order := make([]album.CuratorItem, 0, len(items)+len(added))
	for _, it := range items {
		if it.Removed || removed[it.PhotoID] {
			continue
		}
		if name, ok := names[it.PhotoID]; ok {
			it.SortFileName = name
		}
		order = append(order, it)
	}
	withAdded, err := app.albumItemsForSort(year, nil, added)
	if err != nil {
		return err
	}
	for _, it := range withAdded {
		if name, ok := names[it.PhotoID]; ok {
			it.SortFileName = name
		}
		order = append(order, it)
	}
	resorted, err := app.applyAlbumSortMode(year, albumID, album.SortModeOr(header.SortMode), order)
	if err != nil {
		return err
	}
	if resorted != nil {
		res.Resorted++
	}
	return nil
}

// diplomaFileName names a patrol's photograph after its number, zero-padded to three digits so a filename sort is a
// number sort: `7` becomes `007.jpg`. "" for a patrol with no number yet, which then sorts first.
func diplomaFileName(number, contentType string) string {
	number = strings.TrimSpace(number)
	if number == "" {
		return ""
	}
	if len(number) < 3 && strings.Trim(number, "0123456789") == "" {
		number = strings.Repeat("0", 3-len(number)) + number
	}
	switch contentType {
	case "image/png":
		return number + ".png"
	case "image/webp":
		return number + ".webp"
	default:
		// foto's display renditions are JPEG.
		return number + ".jpg"
	}
}

// diplomaAlbumID finds a type's album by its slug, or creates it as a draft sorted by filename.
//
// Found by slug rather than remembered, so there is nothing to store: the slug is frozen at creation, and a curator
// who renames the album keeps the one the sync fills. A deleted album with the slug is refused by createAdminAlbum,
// and the sync reports that rather than silently recreating it.
func (app *application) diplomaAlbumID(year, title, description string) (string, error) {
	slug := slugifyAlbumTitle(title)
	all, err := app.models.AlbumCurator.All(year)
	if err != nil {
		return "", err
	}
	for _, a := range all {
		if a.Slug == slug && !a.Deleted {
			return a.ID, nil
		}
	}
	albumID, _, err := app.createAdminAlbum(year, title, description, 0)
	if err != nil {
		return "", err
	}
	// createAdminAlbum makes every album time-sorted; these read in patrol order. Set once, at creation, so a
	// curator who picks another mode keeps it.
	mode := album.SortModeFilenameAsc
	if err := app.publishAlbum(year, album.VerbUpdated, albumID, album.Updated{
		AlbumID:   albumID,
		Year:      year,
		SortMode:  &mode,
		UpdatedAt: time.Now().UTC(),
	}); err != nil {
		return "", err
	}
	app.waitForAdminAlbum(year, albumID)
	return albumID, nil
}

// diplomaPhotoUploaded brings a photograph's bytes into the store and describes it as a library photograph.
//
// The thumbnail is best-effort, as on every other upload path: the library falls back to the full image.
func (app *application) diplomaPhotoUploaded(ctx context.Context, year, ref, thumbRef string, width, height int, fileName string) (photo.Uploaded, error) {
	data, err := app.photos.Bytes(ctx, blob.Ref(ref))
	if err != nil {
		return photo.Uploaded{}, err
	}
	if thumbRef != "" {
		if _, terr := app.photos.Bytes(ctx, blob.Ref(thumbRef)); terr != nil {
			app.Logger.Info("a patrol photograph's thumbnail is unavailable", "ref", thumbRef, "err", terr)
			thumbRef = ""
		}
	}
	return photo.Uploaded{
		PhotoID:    ref,
		Year:       year,
		Ref:        ref,
		ThumbRef:   thumbRef,
		Width:      width,
		Height:     height,
		Bytes:      len(data),
		FileName:   fileName,
		UploadedAt: time.Now().UTC(),
	}, nil
}

// photoTaggedWith reports whether a library photograph already carries a patrol's tag.
func (app *application) photoTaggedWith(year, photoID, teamID string) (bool, error) {
	tags, err := app.models.PhotoCurator.Tags(year, photoID)
	if err != nil {
		return false, err
	}
	for _, tag := range tags {
		if tag.TeamID == teamID {
			return true, nil
		}
	}
	return false, nil
}

func (app *application) publishPhoto(year, photoID, verb string, body any) error {
	subject, err := photo.Subject(year, photoID, verb)
	if err != nil {
		return err
	}
	return app.commands.Publish(subject, body)
}

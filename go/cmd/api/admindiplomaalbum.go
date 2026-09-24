package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"nathejk.dk/internal/blob"
	"nathejk.dk/nathejk/table/album"
	"nathejk.dk/nathejk/table/photo"
)

// The diploma photographs, as an album (task 397).
//
// # What it does
//
// One curator button on the album list. For every patrol of the year that the diploma has a photograph for, it
// takes **that** photograph — `patrolphoto.Cover`, the one read the diploma uses, so the two cannot disagree —
// puts it in the library, tags it with the patrol's number, and files it in one draft album, "Diplombilleder".
//
// # Consent
//
// A patrol whose Fototilladelse records a refusal has no cover, so it is never listed and its photograph never
// reaches the library — the same gate the diploma has. Each press also first takes every refusing patrol's
// photographs out of all albums, catching refusals that arrived while the app was down (see photoconsent.go).
//
// # Safe to press twice
//
// Every step is keyed by content: the library id is the photograph's ref, a tag is keyed by photo and team, and a
// photograph already live in the album is skipped. A second press adds only what changed — a patrol photographed
// since, or a cover somebody chose in hq — and never undoes a curator's work: a photograph deleted from the
// library stays deleted, one removed from the album stays out, and the album stays a draft until somebody
// publishes it.
//
// # No re-encode
//
// The bytes are foto's display rendition, which foto rebuilt from pixels and which therefore carries no EXIF (see
// patrolphoto/table.sql). They are fetched and hash-verified by `internal/photobytes` into the same blob store the
// library reads, so the library id is the ref the diploma already uses, and nothing is stored twice.

const (
	diplomaAlbumTitle = "Diplombilleder"
	// diplomaAlbumTimeout bounds one press. A year is a few hundred patrols, most of whose photographs are
	// already in the store from rendering diplomas; the rest are one fetch from foto each.
	diplomaAlbumTimeout = 5 * time.Minute
)

// diplomaAlbumResult is what one press did, for the note and the log.
type diplomaAlbumResult struct {
	Patrols  int
	Uploaded int
	Tagged   int
	Added    int
	// Removed are album memberships taken down because the patrol refused photographs.
	Removed int
	// Skipped are patrols whose photograph could not be fetched, or was deleted from the library.
	Skipped int
}

// createDiplomaAlbumFragmentHandler runs the action and re-renders the album list with what it did.
func (app *application) createDiplomaAlbumFragmentHandler(w http.ResponseWriter, r *http.Request) {
	if app.models.AlbumCurator == nil || app.models.PhotoCurator == nil ||
		app.models.PatrolPhotos == nil || app.photos == nil {
		app.ServiceUnavailableResponse(w, r, "diplombillederne er ikke tilgængelige lige nu")
		return
	}

	if rc := http.NewResponseController(w); rc != nil {
		if err := rc.SetWriteDeadline(time.Now().Add(diplomaAlbumTimeout)); err != nil {
			app.Logger.Warn("could not extend the diploma album write deadline", "err", err)
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), diplomaAlbumTimeout)
	defer cancel()

	res, err := app.fileDiplomaPhotos(ctx, adminYear(r))
	if err != nil {
		var cerr *adminAlbumCreateError
		if errors.As(err, &cerr) && cerr.Kind == adminAlbumCreateRefused {
			app.renderAdminAlbumList(w, r, upperFirst(cerr.Err.Error())+".")
			return
		}
		app.Logger.Error("filing the diploma photographs", "year", adminYear(r), "err", err,
			"uploaded", res.Uploaded, "tagged", res.Tagged, "added", res.Added)
		app.renderAdminAlbumList(w, r, "Kunne ikke lægge diplombillederne i album. Prøv igen.")
		return
	}

	app.Logger.Info("admin filed the diploma photographs",
		"year", adminYear(r), "patrols", res.Patrols, "uploaded", res.Uploaded, "tagged", res.Tagged,
		"added", res.Added, "removed", res.Removed, "skipped", res.Skipped, "ip", clientIP(r))
	app.renderAdminAlbumList(w, r, diplomaAlbumMessage(res))
}

// fileDiplomaPhotos does the work for one year. The result counts what was done even when it returns an error.
func (app *application) fileDiplomaPhotos(ctx context.Context, year string) (diplomaAlbumResult, error) {
	var res diplomaAlbumResult

	// Refusals first: one that arrived while the app was down was never reacted to (see photoconsent.go), and
	// this is the moment a curator is looking at the albums.
	refused, err := app.models.PatrolPhotos.Refused(year)
	if err != nil {
		return res, err
	}
	for _, teamID := range refused {
		n, err := app.removeRefusedFromAlbums(year, teamID)
		res.Removed += n
		if err != nil {
			return res, err
		}
	}

	teams, err := app.models.PatrolPhotos.Teams(year)
	if err != nil {
		return res, err
	}
	res.Patrols = len(teams)
	if len(teams) == 0 {
		return res, nil
	}

	albumID, err := app.diplomaAlbumID(year)
	if err != nil {
		return res, err
	}
	_, items, found, err := app.models.AlbumCurator.Album(year, albumID)
	if err != nil {
		return res, err
	}
	if !found {
		return res, fmt.Errorf("the diploma album %s is not in the projection yet", albumID)
	}
	// Every membership counts, removed ones included: a photograph a curator took out of this album stays out.
	inAlbum := make(map[string]bool, len(items))
	for _, it := range items {
		inAlbum[it.PhotoID] = true
	}
	next, err := app.models.AlbumCurator.NextOrdinal(year, albumID)
	if err != nil {
		return res, err
	}

	for _, t := range teams {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		cover, ok, err := app.models.PatrolPhotos.Cover(year, t.TeamID)
		if err != nil {
			return res, err
		}
		if !ok {
			// Consent withdrawn or the photograph purged between the two reads.
			continue
		}
		photoID := cover.Ref

		existing, known, err := app.models.PhotoCurator.Photo(year, photoID)
		if err != nil {
			return res, err
		}
		if known && existing.Deleted {
			// A curator took it down, possibly because somebody asked. Not this button's decision to reverse.
			res.Skipped++
			continue
		}
		if !known {
			uploaded, err := app.diplomaPhotoUploaded(ctx, year, cover.Ref, cover.ThumbRef, cover.Width, cover.Height)
			if err != nil {
				app.Logger.Info("a diploma photograph could not be fetched for the library",
					"team", t.TeamID, "ref", cover.Ref, "err", err)
				res.Skipped++
				continue
			}
			if err := app.publishPhoto(year, photoID, photo.VerbUploaded, uploaded); err != nil {
				return res, err
			}
			res.Uploaded++
		}

		if t.Number != "" {
			tagged, err := app.photoTaggedWith(year, photoID, t.TeamID)
			if err != nil {
				return res, err
			}
			if !tagged {
				if err := app.publishPhoto(year, photoID, photo.VerbPatrolTagged, photo.PatrolTagged{
					PhotoID:  photoID,
					Year:     year,
					TeamID:   t.TeamID,
					Number:   t.Number,
					TaggedAt: time.Now().UTC(),
				}); err != nil {
					return res, err
				}
				res.Tagged++
			}
		}

		if !inAlbum[photoID] {
			if err := app.publishAlbum(year, album.VerbItemAdded, albumID, album.ItemAdded{
				AlbumID: albumID,
				Year:    year,
				Ordinal: next,
				PhotoID: photoID,
				AddedAt: time.Now().UTC(),
			}); err != nil {
				return res, err
			}
			inAlbum[photoID] = true
			next++
			res.Added++
		}
	}
	return res, nil
}

// diplomaAlbumID finds the year's diploma album by its slug, or creates it as a draft.
//
// Found by slug rather than remembered, so there is nothing to store: the slug is frozen at creation, and a
// curator who renames the album keeps the one this button fills. A deleted album with the slug is refused by
// createAdminAlbum with a sentence the list shows, rather than silently recreated.
func (app *application) diplomaAlbumID(year string) (string, error) {
	slug := slugifyAlbumTitle(diplomaAlbumTitle)
	all, err := app.models.AlbumCurator.All(year)
	if err != nil {
		return "", err
	}
	for _, a := range all {
		if a.Slug == slug && !a.Deleted {
			return a.ID, nil
		}
	}
	albumID, _, err := app.createAdminAlbum(year, diplomaAlbumTitle,
		"Patruljernes billeder fra diplomerne, tagget med patruljenummer.", 0)
	if err != nil {
		return "", err
	}
	app.waitForAdminAlbum(year, albumID)
	return albumID, nil
}

// diplomaPhotoUploaded brings a cover's bytes into the store and describes it as a library photograph.
//
// The thumbnail is best-effort, as on every other upload path: the library falls back to the full image.
func (app *application) diplomaPhotoUploaded(ctx context.Context, year, ref, thumbRef string, width, height int) (photo.Uploaded, error) {
	data, err := app.photos.Bytes(ctx, blob.Ref(ref))
	if err != nil {
		return photo.Uploaded{}, err
	}
	if thumbRef != "" {
		if _, terr := app.photos.Bytes(ctx, blob.Ref(thumbRef)); terr != nil {
			app.Logger.Info("a diploma photograph's thumbnail is unavailable", "ref", thumbRef, "err", terr)
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

// diplomaAlbumMessage is the one sentence the album list shows after a press.
func diplomaAlbumMessage(res diplomaAlbumResult) string {
	if res.Patrols == 0 && res.Removed == 0 {
		return "Ingen patruljer har et diplombillede endnu."
	}
	msg := fmt.Sprintf("Diplombilleder: %s lagt i albummet «%s»", photoCount(res.Added), diplomaAlbumTitle)
	if res.Added == 0 {
		msg = fmt.Sprintf("Diplombillederne lå allerede i albummet «%s»", diplomaAlbumTitle)
	}
	if res.Removed > 0 {
		msg += fmt.Sprintf(", %s fjernet fra album fordi patruljen har frabedt sig billeder", photoCount(res.Removed))
	}
	if res.Skipped > 0 {
		msg += fmt.Sprintf(", %d sprunget over (slettet eller ikke til at hente)", res.Skipped)
	}
	return msg + "."
}

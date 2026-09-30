package main

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/julienschmidt/httprouter"

	"nathejk.dk/internal/blob"
	"nathejk.dk/nathejk/table/photo"
)

// The curator's library reads (PRD 022 §7, task 374).
//
// # Two endpoints, and only one of them is dangerous
//
// `GET /api/admin/photos` is a list. `GET /api/admin/photos/{photoId}/media` serves **bytes**, and it is the
// one route in this feature that could be turned into a general file server by a small mistake — see
// `showAdminPhotoMediaHandler`.

// maxAdminLibraryIDs bounds the `ids` filter.
//
// One placeholder per id goes into an `IN (…)`, so the statement's width is the caller's to keep sane. 200 is
// well above the uploader's chunk of 100 and well below anything that would embarrass the driver; a batch larger
// than this asks in several requests, which it has to do anyway to keep each answer prompt.
const maxAdminLibraryIDs = 200

// adminLibraryResponse is one page of the library plus the header's numbers.
//
// The counts travel with the page rather than on their own endpoint. They are shown on the same line as the
// grid and must agree with it, and two requests could each be correct about a different instant — which reads
// to a curator as the tool contradicting itself.
type adminLibraryResponse struct {
	Photos []adminLibraryPhoto `json:"photos"`
	Counts adminCountsView     `json:"counts"`

	// Limit and Offset are echoed so the client does not have to remember what it asked for, and so a
	// clamped limit is visible rather than silently different from the request.
	Limit  int `json:"limit"`
	Offset int `json:"offset"`

	// CoverPhotoID is the album's cover when the read was narrowed to one album (task 396), so the album view
	// can mark it. Not on the wire: it is the page's concern, and the JSON read's contract is the photographs.
	CoverPhotoID string `json:"-"`

	// HasMore is whether another page exists.
	//
	// Derived from asking for one row more than the page size rather than from a COUNT: the count would be a
	// second aggregate over the same filter, and "is there more" is the only question the grid asks.
	HasMore bool `json:"hasMore"`
}

// adminLibraryPhoto is one thumbnail in the contact sheet.
//
// No blob refs. The client addresses bytes through the photo id and a variant, which is what keeps the media
// route's projection check load-bearing — if a ref were here, the client would have no reason to go through it.
type adminLibraryPhoto struct {
	ID      string `json:"id"`
	Caption string `json:"caption,omitempty"`

	// Credit is the photographer's credit line, or absent when there is none (task 393).
	//
	// The one person-shaped field the curator's tool carries, narrowly and deliberately — free text somebody
	// typed, never derived from the `person` projection. See photo/table.sql's column comment for the bounds.
	Credit string `json:"credit,omitempty"`

	// CreditIsCrew says the credit came from the **crew picker** rather than from a typed name (task 473).
	//
	// # Why the kind has to be on the wire when the reference must not be
	//
	// The reference itself stays off it, deliberately (PRD 025 §6 R6): a crew id is person-shaped, and the response
	// carries the resolved name instead. But without knowing *which kind* a credit is, every editor that prefills
	// from `Credit` and saves what it finds converts a reference into a typed string — silently turning the
	// erasable form into the one that cannot be erased, which is the entire point of PRD 025 §8 D1 running
	// backwards. `vieweredit.js` was doing exactly that.
	//
	// A boolean about which column holds the credit names nobody. It is excepted in
	// `libraryPersonShapedExceptions` on that basis, and the exception is narrower than the one the reference would
	// have needed.
	CreditIsCrew bool `json:"creditIsCrew,omitempty"`
	Width        int  `json:"width,omitempty"`
	Height       int  `json:"height,omitempty"`

	// Lat/Lng are omitted when the photograph has no coordinate, which is the common case.
	Lat *float64 `json:"lat,omitempty"`
	Lng *float64 `json:"lng,omitempty"`

	// BoundsVerdict is always present, including "none". Sent even when there is no coordinate because the
	// grid renders the four states distinguishably and an absent field would collapse two of them.
	BoundsVerdict string `json:"boundsVerdict"`

	// AlbumCount and TagCount drive the "already sorted" and "already tagged" marks.
	AlbumCount int `json:"albumCount"`
	TagCount   int `json:"tagCount"`

	Deleted    bool   `json:"deleted,omitempty"`
	UploadedAt string `json:"uploadedAt,omitempty"`
}

// listAdminPhotosHandler returns one page of the year's library.
//
// @Summary      List the year's photograph library
// @Description  Returns one page of the configured event year's photographs, newest first, with the counts the tool's header shows. Filters compose: `album=none` limits to photographs no live album references, `album={albumId}` to one album's live members in the album's own order, `caption=yes|no` to those with or without a caption, `location=yes|no` to those with or without a coordinate, `verdict=inside|outside|unknown|none` to one bounds verdict, `tagged=yes|no` to those with or without a patrol attribution, `credit=none|any` to those without or with a credit of either kind and `credit=<crew id or exact credit line>` to one photographer's (exact, not a search — this library has no search, and a substring filter would be one by the back door), `ids=` a comma-separated list limits to those photographs, and `deleted=1` includes ones the curator removed. `ids` exists for a specific purpose: an upload answers with the photograph's id before the projection this read serves has folded the event, so the uploader polls with the ids it was given until they come back — which is how it knows the contact sheet is worth reloading. This read is **draft-visible** — it returns photographs no album references and, on request, deleted ones — which is why it is on the curator interface and behind the admin credential rather than on any public read. Requires the admin credential.
// @Tags         admin
// @Produce      json
// @Param        album     query     string  false  "none: only photographs in no album; an album id: that album's members, in album order"
// @Param        location  query     string  false  "yes or no: with or without a coordinate"
// @Param        verdict   query     string  false  "inside, outside, unknown or none"
// @Param        caption   query     string  false  "yes or no: with or without a caption"
// @Param        tagged    query     string  false  "yes or no: with or without a patrol tag"
// @Param        credit    query     string  false  "none: no credit at all; any: either kind; or a crew id or an exact credit line"
// @Param        ids       query     string  false  "comma-separated photograph ids, at most 200: which of these the projection can see"
// @Param        deleted   query     int     false  "1 to include deleted photographs"
// @Param        limit     query     int     false  "page size, default 120, max 500"
// @Param        offset    query     int     false  "rows to skip"
// @Success      200  {object}  adminLibraryResponse
// @Failure      400  {object}  map[string]string  "an unrecognised filter value"
// @Failure      401  "missing or wrong admin credential — a plain-text body with a WWW-Authenticate challenge, not the JSON envelope"
// @Failure      421  "the tool was reached over plain HTTP, so the credential in the request is refused unread"
// @Failure      500  {object}  map[string]string
// @Failure      503  {object}  map[string]string  "the library is unavailable"
// @Router       /admin/photos [get]
func (app *application) listAdminPhotosHandler(w http.ResponseWriter, r *http.Request) {
	page, ok := app.readAdminLibraryPage(w, r)
	if !ok {
		return
	}

	if err := app.WriteJSON(w, http.StatusOK, page, nil); err != nil {
		app.ServerErrorResponse(w, r, err)
	}
}

// readAdminLibraryPage validates the filter and paging, reads one page, and answers on failure.
//
// Shared by the JSON endpoint above and the contact sheet's htmx fragment (task 395). **Shared rather than
// reimplemented**, because both interpret the same filter: the grid renders one page of it and "select all
// matching this filter" pages the ids out of the other, and the two disagreeing is precisely how a bulk action
// lands on photographs the curator never saw.
//
// It returns `adminLibraryResponse` although one caller renders HTML. A second, tag-free struct with identical
// fields was the first attempt and staticcheck was right to reject it: two shapes that must stay equal are worse
// than one name that reads slightly oddly at one of its two call sites.
func (app *application) readAdminLibraryPage(w http.ResponseWriter, r *http.Request) (adminLibraryResponse, bool) {
	if app.models.PhotoCurator == nil {
		app.ServiceUnavailableResponse(w, r, "billedarkivet er ikke tilgængeligt lige nu")
		return adminLibraryResponse{}, false
	}

	filter, err := adminLibraryFilter(r)
	if err != nil {
		app.BadRequestResponse(w, r, err)
		return adminLibraryResponse{}, false
	}
	var cover string
	// An album id must name an album of this year, for the reason every other filter value is refused rather than
	// ignored: `album=all`, or a typo, reading as an empty album is a grid the curator misreads. Deleted albums
	// count — their editor still opens.
	if filter.AlbumID != "" {
		if app.models.AlbumCurator == nil {
			app.ServiceUnavailableResponse(w, r, "albummerne er ikke tilgængelige lige nu")
			return adminLibraryResponse{}, false
		}
		a, _, found, err := app.models.AlbumCurator.Album(adminYear(r), filter.AlbumID)
		if err != nil {
			app.ServerErrorResponse(w, r, err)
			return adminLibraryResponse{}, false
		}
		if !found {
			app.BadRequestResponse(w, r, errors.New(`ukendt album i "album"`))
			return adminLibraryResponse{}, false
		}
		cover = a.CoverPhotoID
	}

	limit := adminQueryInt(r, "limit", 120)
	offset := adminQueryInt(r, "offset", 0)
	if limit <= 0 {
		limit = 120
	}
	if limit > 500 {
		limit = 500
	}
	if offset < 0 {
		offset = 0
	}

	// One row more than the page, so "is there another page" costs nothing. A COUNT over the same filter would
	// be a second aggregate for a question the grid answers with a button.
	rows, err := app.models.PhotoCurator.Library(adminYear(r), filter, limit+1, offset)
	if err != nil {
		app.ServerErrorResponse(w, r, err)
		return adminLibraryResponse{}, false
	}

	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}

	counts, err := app.models.PhotoCurator.Counts(adminYear(r))
	if err != nil {
		// The grid is the point of the request and the counts are chrome, so a failing count degrades the
		// header rather than the page. Logged, because a header stuck at zero while the grid fills is the kind
		// of thing a curator reports as "the numbers are wrong" months later.
		app.Logger.Error("reading the library counts", "err", err)
	}

	page := adminLibraryResponse{
		Photos: make([]adminLibraryPhoto, 0, len(rows)),
		Counts: adminCountsView{
			Total:        counts.Total,
			InNoAlbum:    counts.InNoAlbum,
			WithLocation: counts.WithLocation,
			Plottable:    counts.Plottable,
			OutOfBounds:  counts.OutOfBounds,
			Unknown:      counts.Unknown,
			Tagged:       counts.Tagged,
			Deleted:      counts.Deleted,
		},
		Limit:        limit,
		Offset:       offset,
		HasMore:      hasMore,
		CoverPhotoID: cover,
	}
	// Credits resolved for the page, so **the tool shows what the public will show** (PRD 025, task 452).
	// A curator checking their work has to see the resolved name — not the reference, which is meaningless to
	// read, and not the typed field, which is empty on a photograph credited by picker. It is also how a curator
	// discovers that a credit has stopped resolving because the crew member asked to be deleted.
	var creditIDs []string
	for _, p := range rows {
		if p.CreditCrewID != "" {
			creditIDs = append(creditIDs, p.CreditCrewID)
		}
	}
	creditNames := app.creditNames(adminYear(r), creditIDs)

	for _, p := range rows {
		page.Photos = append(page.Photos, adminLibraryPhoto{
			ID:      p.ID,
			Caption: p.Caption,
			// The resolved name. The reference itself stays out of the response: it is person-shaped, and
			// `isPersonShaped` flags it precisely so that a *response* type cannot quietly acquire one —
			// `libraryPersonShapedExceptions` admits the column, not the wire (PRD 025 §6 R6).
			Credit:        resolvedCredit(p.Credit, p.CreditCrewID, creditNames),
			CreditIsCrew:  p.CreditCrewID != "",
			Width:         p.Width,
			Height:        p.Height,
			Lat:           p.Lat,
			Lng:           p.Lng,
			BoundsVerdict: p.BoundsVerdict,
			AlbumCount:    p.AlbumCount,
			TagCount:      p.TagCount,
			Deleted:       p.Deleted,
			UploadedAt:    p.UploadedAt,
		})
	}
	return page, true
}

// adminLibraryFilter reads the filter from the query string.
//
// # Composable named values rather than one mode
//
// PRD 022 §6 lists six filters, and a single `filter=` mode would make them mutually exclusive. The curator's
// real question is often a conjunction — "which photographs are in no album *and* have no position yet" is the
// start of the bulk-position workflow — so the parameters compose and the UI offers the six as presets over
// them.
//
// # Unrecognised values are refused, not ignored
//
// A typo'd `verdict=insid` silently ignored would show the curator *everything* while they believed they were
// looking at the plottable subset — and the action bar acts on what is selected. Refusing is the difference
// between a confusing screen and a bulk edit applied to the wrong forty photographs.
func adminLibraryFilter(r *http.Request) (photo.Filter, error) {
	q := r.URL.Query()
	var f photo.Filter

	// `none`, or one album's id (task 396). Bound as a parameter, never spliced, so it is only bounded here;
	// readAdminLibraryPage then refuses one that names no album of this year.
	switch v := q.Get("album"); {
	case v == "":
	case v == "none":
		f.InNoAlbum = true
	case len(v) <= 99 && !strings.ContainsAny(v, " \t\r\n"):
		f.AlbumID = v
	default:
		return f, errors.New(`ukendt værdi for "album" ("none" eller et album-id)`)
	}

	switch v := q.Get("location"); v {
	case "":
	case "yes":
		yes := true
		f.HasLocation = &yes
	case "no":
		no := false
		f.HasLocation = &no
	default:
		return f, errors.New(`ukendt værdi for "location" (kun "yes" eller "no")`)
	}

	switch v := q.Get("verdict"); v {
	case "":
	case photo.BoundsInside, photo.BoundsOutside, photo.BoundsUnknown, photo.BoundsNone:
		f.Verdict = v
	default:
		return f, errors.New(`ukendt værdi for "verdict"`)
	}

	// `caption`: yes or no, like `location` and unlike `credit` (task 460).
	//
	// `yes`/`no` rather than `any`/`none` because this parameter carries no values: a caption is prose, and
	// "which photographs say exactly this" is not a question anybody asks. `credit` spells its sentinels as words
	// precisely because it *does* carry a value — and for the day somebody is credited as "no".
	switch v := q.Get("caption"); v {
	case "":
	case "yes":
		yes := true
		f.HasCaption = &yes
	case "no":
		no := false
		f.HasCaption = &no
	default:
		return f, errors.New(`ukendt værdi for "caption" (kun "yes" eller "no")`)
	}

	switch v := q.Get("tagged"); v {
	case "":
	case "yes":
		yes := true
		f.Tagged = &yes
	case "no":
		no := false
		f.Tagged = &no
	default:
		return f, errors.New(`ukendt værdi for "tagged" (kun "yes" eller "no")`)
	}

	// `credit`: none, any, or one specific credit — a crew id or an exact line (task 454).
	//
	// `none` and `any` are words rather than `no`/`yes` like `location` above, and the inconsistency is
	// deliberate: this parameter also carries *values*, so `credit=no` would be ambiguous the day somebody is
	// credited as "no". `album` already spells its sentinel `none` for the same reason.
	switch v := q.Get("credit"); {
	case v == "":
	case v == "none":
		no := false
		f.HasCredit = &no
	case v == "any":
		yes := true
		f.HasCredit = &yes
	case len(v) <= maxAdminCredit && !strings.ContainsAny(v, "\t\r\n"):
		f.CreditIs = v
	default:
		return f, errors.New(`ubrugelig værdi for "credit"`)
	}

	f.IncludeDeleted = q.Get("deleted") == "1"

	// `ids`: which of these photographs the projection can see (task 438).
	//
	// Refused rather than clamped, like every other filter value here and for the same reason: a silently
	// shortened list answers a different question from the one asked, and the caller would read the answer as
	// "those are not there yet" and wait for something that will never arrive.
	if raw := q.Get("ids"); raw != "" {
		for _, id := range strings.Split(raw, ",") {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			if len(id) > 99 || strings.ContainsAny(id, " \t\r\n") {
				return f, errors.New(`ubrugeligt id i "ids"`)
			}
			f.PhotoIDs = append(f.PhotoIDs, id)
		}
		if len(f.PhotoIDs) > maxAdminLibraryIDs {
			return f, fmt.Errorf(`for mange id'er i "ids" (højst %d)`, maxAdminLibraryIDs)
		}
	}

	return f, nil
}

// adminQueryInt reads an integer query parameter, falling back rather than erroring.
//
// A malformed `limit` is a client bug with an obvious safe answer, unlike a malformed *filter*, which changes
// which photographs the curator is looking at. The asymmetry is deliberate: one is cosmetic, the other decides
// what a bulk action applies to.
func adminQueryInt(r *http.Request, key string, fallback int) int {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return n
}

// showAdminPhotoMediaHandler streams one photograph's bytes.
//
// @Summary      Serve a library photograph's bytes
// @Description  Streams one photograph from the year's library, as a thumbnail with `?variant=thumb`, the 800px rendition with `?variant=medium`, or the full stored rendition otherwise. A variant whose rendition was never produced falls back to the full image rather than 404-ing. The id is resolved through the library projection rather than being handed to the blob store, so this route serves photographs of the configured year and nothing else. A deleted photograph answers 404. Requires the admin credential.
// @Tags         admin
// @Produce      image/jpeg
// @Param        photoId  path      string  true   "library photograph id"
// @Param        variant  query     string  false  "thumb for the 320px thumbnail, medium for the 800px rendition; omit for the full rendition"
// @Success      200  {file}  binary
// @Failure      304  "not modified: the browser already holds these bytes. A rendition is immutable, so its id is its content hash and a revalidation can always be answered without reading the object."
// @Failure      400  {object}  map[string]string  "no working year, or one the tool does not know (X-Admin-Year or ?year=)"
// @Failure      401  "missing or wrong admin credential — a plain-text body with a WWW-Authenticate challenge, not the JSON envelope"
// @Failure      421  "the tool was reached over plain HTTP, so the credential in the request is refused unread"
// @Failure      404  {object}  map[string]string  "unknown or deleted photograph"
// @Failure      500  {object}  map[string]string
// @Failure      503  {object}  map[string]string  "the library is unavailable"
// @Router       /admin/photos/{photoId}/media [get]
//
// # This is the one route here that could become a file server
//
// The rule PRD 022 §8.4 inherits from `glimtmediaserve.go` is that **blobs are never addressed by ref in a
// URL**: the path names an entity, a projection read applies the visibility filter, and only then does a
// `blob.Ref` appear.
//
// That rule is easy to *appear* to follow here and easy to actually break, because a library photograph's id
// **is** its content ref. Passing the path segment to the blob store would work perfectly for every real
// photograph — and would also serve any other object in the store to anyone holding the shared password: a
// participant's portrait, a glimt somebody took down, a diploma. One password, one URL shape, the whole store.
//
// So the id goes through `PhotoCurator.Photo` first, and the ref used is the one that came **out of the row**,
// never the one that came in off the wire. `TestAdminMediaRefusesARefThatIsNotARow` is the regression test, and
// it is the most important test on this surface.
func (app *application) showAdminPhotoMediaHandler(w http.ResponseWriter, r *http.Request) {
	if app.models.PhotoCurator == nil {
		app.ServiceUnavailableResponse(w, r, "billedarkivet er ikke tilgængeligt lige nu")
		return
	}

	photoID := httprouter.ParamsFromContext(r.Context()).ByName("photoId")

	p, found, err := app.models.PhotoCurator.Photo(adminYear(r), photoID)
	if err != nil {
		app.ServerErrorResponse(w, r, err)
		return
	}
	// A deleted photograph is found by this read — that is what the curator interface is for — but its bytes
	// are not served. A takedown that still answered on a URL would be a takedown in name only, and the
	// contact sheet shows a deleted row without needing its pixels.
	if !found || p.Deleted {
		app.NotFoundResponse(w, r)
		return
	}

	ref := p.Ref
	edge := 0
	switch r.URL.Query().Get("variant") {
	case "thumb":
		if p.ThumbRef != "" {
			ref, edge = p.ThumbRef, glimtThumbEdges[0]
		}
	case "medium":
		if p.MediumRef != "" {
			ref, edge = p.MediumRef, mediumEdge
		}
	}
	// Validated even though it came out of our own row, because a ref is the one string here that becomes a
	// filesystem path — the same belt-and-braces `albumItemRef` applies.
	stored := blob.Ref(ref)
	if !stored.Valid() {
		app.Logger.Error("a library row holds an unusable ref", "photoId", photoID, "ref", ref)
		app.NotFoundResponse(w, r)
		return
	}

	// The repair plan (task 430). It matters more on this surface than anywhere else: the contact sheet is
	// one thumbnail request per photograph, so a curator opening a full library after a restore is the
	// single largest burst of rebuildable misses the service will ever see — which is exactly what the
	// single-flight in repairRendition is for.
	plan := renditionRepair{}
	if edge > 0 {
		if full := blob.Ref(p.Ref); full.Valid() {
			plan = renditionRepair{
				Target:  stored,
				Source:  full,
				Edge:    edge,
				Quality: glimtJPEGQuality,
			}
		}
	}

	// `no-store`, not the year-long immutable cache the public media route uses. The bytes are immutable, so
	// caching them would be safe in the ordinary sense — but this is an admin surface and PRD 022 §6 requires
	// every response on it to be unstorable: a contact sheet of the event's photographs left in a shared
	// laptop's disk cache outlives the session that fetched it.
	app.streamGlimtMedia(w, r, stored, photoID, "no-store", plan)
}

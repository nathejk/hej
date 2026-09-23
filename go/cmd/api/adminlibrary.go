package main

import (
	"errors"
	"net/http"
	"strconv"

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
	Width   int    `json:"width,omitempty"`
	Height  int    `json:"height,omitempty"`

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
// @Description  Returns one page of the configured event year's photographs, newest first, with the counts the tool's header shows. Filters compose: `album=none` limits to photographs no live album references, `location=yes|no` to those with or without a coordinate, `verdict=inside|outside|unknown|none` to one bounds verdict, `tagged=yes|no` to those with or without a patrol attribution, and `deleted=1` includes ones the curator removed. This read is **draft-visible** — it returns photographs no album references and, on request, deleted ones — which is why it is on the curator interface and behind the admin credential rather than on any public read. Requires the admin credential.
// @Tags         admin
// @Produce      json
// @Param        album     query     string  false  "none: only photographs in no album"
// @Param        location  query     string  false  "yes or no: with or without a coordinate"
// @Param        verdict   query     string  false  "inside, outside, unknown or none"
// @Param        tagged    query     string  false  "yes or no: with or without a patrol tag"
// @Param        deleted   query     int     false  "1 to include deleted photographs"
// @Param        limit     query     int     false  "page size, default 120, max 500"
// @Param        offset    query     int     false  "rows to skip"
// @Success      200  {object}  adminLibraryResponse
// @Failure      400  {object}  map[string]string  "an unrecognised filter value"
// @Failure      401  {object}  map[string]string  "missing or wrong admin credential"
// @Failure      429  {object}  map[string]string  "too many credential attempts from this address"
// @Failure      500  {object}  map[string]string
// @Failure      503  {object}  map[string]string  "the library is unavailable"
// @Router       /admin/photos [get]
func (app *application) listAdminPhotosHandler(w http.ResponseWriter, r *http.Request) {
	if app.models.PhotoCurator == nil {
		app.ServiceUnavailableResponse(w, r, "billedarkivet er ikke tilgængeligt lige nu")
		return
	}

	filter, err := adminLibraryFilter(r)
	if err != nil {
		app.BadRequestResponse(w, r, err)
		return
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
	rows, err := app.models.PhotoCurator.Library(app.config.eventYear, filter, limit+1, offset)
	if err != nil {
		app.ServerErrorResponse(w, r, err)
		return
	}

	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}

	counts, err := app.models.PhotoCurator.Counts(app.config.eventYear)
	if err != nil {
		// The grid is the point of the request and the counts are chrome, so a failing count degrades the
		// header rather than the page. Logged, because a header stuck at zero while the grid fills is the kind
		// of thing a curator reports as "the numbers are wrong" months later.
		app.Logger.Error("reading the library counts", "err", err)
	}

	out := adminLibraryResponse{
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
		Limit:   limit,
		Offset:  offset,
		HasMore: hasMore,
	}
	for _, p := range rows {
		out.Photos = append(out.Photos, adminLibraryPhoto{
			ID:            p.ID,
			Caption:       p.Caption,
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

	if err := app.WriteJSON(w, http.StatusOK, out, nil); err != nil {
		app.ServerErrorResponse(w, r, err)
	}
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

	switch v := q.Get("album"); v {
	case "":
	case "none":
		f.InNoAlbum = true
	default:
		return f, errors.New(`ukendt værdi for "album" (kun "none")`)
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

	f.IncludeDeleted = q.Get("deleted") == "1"

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
// @Description  Streams one photograph from the year's library, as a thumbnail with `?variant=thumb` or the full stored rendition otherwise. The id is resolved through the library projection rather than being handed to the blob store, so this route serves photographs of the configured year and nothing else. A deleted photograph answers 404. Requires the admin credential.
// @Tags         admin
// @Produce      image/jpeg
// @Param        photoId  path      string  true   "library photograph id"
// @Param        variant  query     string  false  "thumb for the thumbnail; omit for the full rendition"
// @Success      200  {file}  binary
// @Failure      401  {object}  map[string]string  "missing or wrong admin credential"
// @Failure      404  {object}  map[string]string  "unknown or deleted photograph"
// @Failure      429  {object}  map[string]string  "too many credential attempts from this address"
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

	p, found, err := app.models.PhotoCurator.Photo(app.config.eventYear, photoID)
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
	if r.URL.Query().Get("variant") == "thumb" && p.ThumbRef != "" {
		ref = p.ThumbRef
	}
	// Validated even though it came out of our own row, because a ref is the one string here that becomes a
	// filesystem path — the same belt-and-braces `albumItemRef` applies.
	stored := blob.Ref(ref)
	if !stored.Valid() {
		app.Logger.Error("a library row holds an unusable ref", "photoId", photoID, "ref", ref)
		app.NotFoundResponse(w, r)
		return
	}

	// `no-store`, not the year-long immutable cache the public media route uses. The bytes are immutable, so
	// caching them would be safe in the ordinary sense — but this is an admin surface and PRD 022 §6 requires
	// every response on it to be unstorable: a contact sheet of the event's photographs left in a shared
	// laptop's disk cache outlives the session that fetched it.
	app.streamGlimtMedia(w, r, stored, photoID, "no-store")
}

package main

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"nathejk.dk/internal/commands"
	"nathejk.dk/internal/users"
	"nathejk.dk/nathejk/table/glimt"
)

// Creating a glimt and reading the feed (PRD 019 §8, task 304).

// maxGlimtCaption is the caption ceiling, in characters (PRD 019 §6).
//
// Counted in **runes, not bytes**: the captions are Danish, so æ/ø/å cost two bytes each in UTF-8,
// and a byte limit would silently give a member writing "på vej gennem skoven" a shorter caption
// than one writing in ASCII. 280 characters is the requirement; 280 bytes would be a different and
// worse one.
const maxGlimtCaption = 280

// maxGlimtItems is the most media one glimt may carry (PRD 019 §6).
const maxGlimtItems = 10

// createGlimtRequest is what the composer posts once its media are uploaded.
type createGlimtRequest struct {
	Caption string `json:"caption"`
	// Audience is group | nathejk | public. **Absent means group** — the narrowest — so a client
	// that forgets the field cannot accidentally publish to the open web. Widening is only ever
	// the result of an explicit value.
	Audience string `json:"audience"`
	// Media are the refs returned by POST /api/glimt/media, in the order the author arranged
	// them.
	Media []createGlimtMedia `json:"media"`
}

type createGlimtMedia struct {
	Ref      string `json:"ref"`
	ThumbRef string `json:"thumb_ref"`
	Kind     string `json:"kind"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	Bytes    int    `json:"bytes"`
	// DurationMs is only meaningful for video, and is re-derived server-side rather than
	// trusted (task 322): a recorder UI's own cap is not a cap.
	DurationMs int `json:"duration_ms"`
}

// createGlimtHandler publishes a new glimt.
//
// @Summary      Share a glimt
// @Description  Creates a glimt from media already uploaded via POST /glimt/media. Caption is optional and capped at 280 characters (counted as characters, not bytes). Audience is one of `group` (the author's own spejder/bandit/crew group), `nathejk` (every authenticated member) or `public` (the open web, via /offentligt/glimt); an absent audience defaults to the narrowest, `group`. Audience is immutable afterwards — widening it would retroactively expose a photo shared under a narrower promise, so there is no update endpoint. Between 1 and 10 media items, whose refs must be ones this API returned. The hold attribution (number, name, group) is frozen from the author's record at this moment and never re-derived.
// @Tags         glimt
// @Accept       json
// @Produce      json
// @Param        request  body      createGlimtRequest  true  "caption, audience and media refs"
// @Success      201  {object}  glimtResponse
// @Failure      400  {object}  map[string]string  "bad caption, audience, or media refs"
// @Failure      401  {object}  map[string]string
// @Failure      404  {object}  map[string]string  "the caller's own record could not be resolved"
// @Failure      429  {object}  map[string]string
// @Failure      503  {object}  map[string]string  "event stream unavailable — retry"
// @Router       /glimt [post]
func (app *application) createGlimtHandler(w http.ResponseWriter, r *http.Request) {
	s, ok := contextGetSession(r)
	if !ok {
		app.AuthenticationRequiredResponse(w, r)
		return
	}

	if app.glimtLimiter != nil && !app.glimtLimiter.Allow(s.UserID) {
		app.RateLimitMessageResponse(w, r,
			"Du har delt mange glimt på kort tid. Prøv igen om lidt.")
		return
	}

	var in createGlimtRequest
	if err := app.ReadJSON(w, r, &in); err != nil {
		app.BadRequestResponse(w, r, err)
		return
	}

	caption, err := validGlimtCaption(in.Caption)
	if err != nil {
		app.BadRequestResponse(w, r, err)
		return
	}

	audience, err := validGlimtAudience(in.Audience)
	if err != nil {
		app.BadRequestResponse(w, r, err)
		return
	}

	media, err := validGlimtMedia(in.Media)
	if err != nil {
		app.BadRequestResponse(w, r, err)
		return
	}

	// The attribution is resolved **now** and frozen (task 302). Everything after this reads
	// the stored row, so a rename or a transfer later cannot rewrite what the moment said.
	viewer, found := app.models.Users.Get(s.UserID)
	if !found {
		// A session whose user no longer resolves. 404 rather than posting an unattributed
		// glimt: a glimt with no hold cannot be shown, and one with no author cannot be
		// deleted by the person who posted it.
		app.NotFoundResponse(w, r)
		return
	}
	p, _ := app.person(s.UserID)
	group, teamNumber, teamName := attributionFor(p, viewer.Role)
	if group == "" {
		// An unrecognised role has no group, and the group is what decides who a
		// `group`-scoped glimt reaches. Refused rather than defaulted to crew, which would
		// put an unclassifiable author's photo in front of the crew group (task 302).
		app.BadRequestResponse(w, r,
			errors.New("din rolle kan ikke afgøre hvem et glimt deles med"))
		return
	}

	glimtID := uuid.NewString()
	subject, serr := glimt.Subject(app.config.eventYear, glimtID, glimt.VerbCreated)
	if serr != nil {
		// A configured year that cannot be a subject token is our problem, not the
		// caller's — and publishing it anyway would make this glimt impossible to hide or
		// delete later, since it would no longer match the per-glimt patterns.
		app.ServerErrorResponse(w, r, serr)
		return
	}

	body := glimt.Created{
		GlimtID:        glimtID,
		Year:           app.config.eventYear,
		AuthorPersonID: s.UserID,
		AuthorGroup:    group,
		TeamNumber:     teamNumber,
		TeamName:       teamName,
		Audience:       audience,
		Caption:        caption,
		Media:          media,
		CreatedAt:      time.Now().UTC(),
	}

	if perr := app.commands.Publish(subject, body); perr != nil {
		if errors.Is(perr, commands.ErrNoPublisher) {
			// 503 and not a fake success. The client keeps the glimt in its outbox and
			// retries on the next foreground (task 314) — which is only safe because we
			// told it the truth here.
			app.ServiceUnavailableResponse(w, r, "kunne ikke dele glimtet, prøv igen")
			return
		}
		app.ServerErrorResponse(w, r, perr)
		return
	}

	// Echoed back from the event body rather than read back from the projection. The fold is
	// asynchronous, so a read here would usually miss — and the client needs the id and the
	// attribution immediately to replace its optimistic card.
	out := newGlimtResponse(glimt.Glimt{
		GlimtID:        glimtID,
		Year:           body.Year,
		AuthorPersonID: body.AuthorPersonID,
		AuthorGroup:    body.AuthorGroup,
		TeamNumber:     body.TeamNumber,
		TeamName:       body.TeamName,
		Audience:       body.Audience,
		Caption:        body.Caption,
		CreatedAt:      body.CreatedAt,
		MediaCount:     len(media),
		Media:          media,
	}, s.UserID)

	if err := app.WriteJSON(w, http.StatusCreated, out, nil); err != nil {
		app.ServerErrorResponse(w, r, err)
	}
}

// validGlimtCaption trims and length-checks a caption.
func validGlimtCaption(in string) (string, error) {
	caption := strings.TrimSpace(in)
	if len([]rune(caption)) > maxGlimtCaption {
		return "", errors.New("teksten er for lang")
	}
	return caption, nil
}

// validGlimtAudience defaults an absent audience to the narrowest and rejects anything unknown.
//
// Defaulting rather than requiring the field is a deliberate asymmetry: the *safe* value is the one
// you get for free, and an unrecognised value is refused outright rather than falling back. A client
// that sends "everyone" by mistake must not be quietly treated as having said `public`, and one that
// sends nothing must not be quietly treated as having said anything but `group`.
func validGlimtAudience(in string) (string, error) {
	if in == "" {
		return glimt.AudienceGroup, nil
	}
	if !users.GlimtAudience(in).Valid() {
		return "", errors.New("ukendt modtagerkreds")
	}
	return in, nil
}

// validGlimtMedia checks the count, the refs and the ordering.
func validGlimtMedia(in []createGlimtMedia) ([]glimt.Media, error) {
	if len(in) == 0 {
		return nil, errors.New("et glimt skal have mindst ét billede")
	}
	if len(in) > maxGlimtItems {
		return nil, errors.New("et glimt kan højst have 10 filer")
	}

	seen := map[string]bool{}
	out := make([]glimt.Media, 0, len(in))
	for i, m := range in {
		ref, ok := glimtBlobRef(m.Ref)
		if !ok {
			// Rejected before it can reach a SQL statement or a URL path. Note this is
			// stricter than blob.Ref.Valid — see glimtBlobRef for the silent-data-loss
			// bug that makes the canonical form a requirement.
			return nil, errors.New("en af filerne kunne ikke genkendes")
		}
		if seen[ref.String()] {
			// The same object twice in one glimt is a client bug — the composer
			// de-duplicates — and allowing it would show the same photo twice with no
			// way for the member to tell which to remove.
			return nil, errors.New("den samme fil er med to gange")
		}
		seen[ref.String()] = true

		thumbRef := ""
		if m.ThumbRef != "" {
			// A thumbnail that cannot be addressed is dropped rather than failing the
			// post: the client falls back to the full item (task 302).
			if tr, tok := glimtBlobRef(m.ThumbRef); tok {
				thumbRef = tr.String()
			}
		}

		kind := glimt.MediaKindImage
		if m.Kind == glimt.MediaKindVideo {
			kind = glimt.MediaKindVideo
		}

		out = append(out, glimt.Media{
			// Assigned from the request order rather than taken from the client. The
			// order is what the author arranged, and it is expressed by the array; a
			// client-supplied ordinal would be a second source of truth for the same
			// fact, free to disagree with the array it arrived in.
			Ordinal:    i,
			Ref:        ref.String(),
			ThumbRef:   thumbRef,
			Kind:       kind,
			Width:      m.Width,
			Height:     m.Height,
			Bytes:      m.Bytes,
			DurationMs: m.DurationMs,
		})
	}
	return out, nil
}

// listGlimtHandler serves the caller's feed, newest first.
//
// @Summary      The glimt shared with you
// @Description  Chronological feed, newest first, of every glimt visible to the caller: their own group's, everything shared with all of Nathejk, everything public, and always their own. Paginated with `limit` (default 20, max 200) and `offset`; page until a short page comes back, as there is deliberately no total count. Media carry no content hashes — each item is fetched from /glimt/{id}/media/{ordinal}, which re-checks visibility. Authors are never named: a glimt is attributed to its hold.
// @Tags         glimt
// @Produce      json
// @Param        limit   query     int  false  "page size (default 20, max 200)"
// @Param        offset  query     int  false  "rows to skip"
// @Success      200  {object}  glimtFeedResponse
// @Failure      401  {object}  map[string]string
// @Failure      503  {object}  map[string]string  "the feed is unavailable"
// @Router       /glimt/feed [get]
func (app *application) listGlimtHandler(w http.ResponseWriter, r *http.Request) {
	s, ok := contextGetSession(r)
	if !ok {
		app.AuthenticationRequiredResponse(w, r)
		return
	}
	if app.models.Glimt == nil {
		// Unavailable, not empty. An empty feed is a legitimate state the client caches, so
		// reporting one because the database is down would leave a device showing "nothing
		// was shared" for the rest of the event.
		app.ServiceUnavailableResponse(w, r, "glimt er ikke tilgængelige lige nu")
		return
	}

	viewer, found := app.models.Users.Get(s.UserID)
	if !found {
		app.NotFoundResponse(w, r)
		return
	}

	limit, offset := pageParams(r)
	filter := app.glimtFilter(s.UserID, viewer.Role)

	rows, err := app.models.Glimt.Feed(app.config.eventYear, filter, limit, offset)
	if err != nil {
		app.ServerErrorResponse(w, r, err)
		return
	}

	out := glimtFeedResponse{
		Glimt: newGlimtResponses(app.visibleGlimt(rows, s.UserID, viewer.Role), s.UserID),
	}
	if err := app.WriteJSON(w, http.StatusOK, out, nil); err != nil {
		app.ServerErrorResponse(w, r, err)
	}
}

// glimtFeedResponse wraps the page.
//
// An object rather than a bare array, so a later addition (a cursor, a count) does not change the
// response's JSON *type* — which would break every client at once.
type glimtFeedResponse struct {
	Glimt []glimtResponse `json:"glimt"`
}

// glimtFilter derives the query narrowing for a caller.
//
// The single conversion point between `users.GlimtFeedFilter` (the authority) and `glimt.Filter`
// (what a query can use). It exists in one place because the two types are the same rule expressed
// twice, and the moment a handler builds a `glimt.Filter` by hand there are two definitions of who
// may see what.
func (app *application) glimtFilter(personID string, role users.Role) glimt.Filter {
	f := users.GlimtFeedFilterFor(app.glimtViewer(personID, role))
	// Note `users.GlimtFeedFilter.IncludeHidden` has no counterpart here, and deliberately so:
	// in SQL terms it is implied by `Unrestricted` (a moderator's clause is unconditional), and
	// a non-moderator's own hidden rows are already covered by the PersonID branch. A second
	// field expressing the same thing would be a second thing to keep in agreement.
	return glimt.Filter{
		Denied:       f.Denied,
		Unrestricted: f.Unrestricted,
		PersonID:     f.PersonID,
		Group:        string(f.Group),
	}
}

// visibleGlimt re-checks every row against the predicate before it is serialised.
//
// Belt and braces, deliberately. The SQL filter already narrowed the query, so this should never
// remove anything — but "should never" is doing a lot of work in a feature whose failure mode is a
// group-scoped photograph of a child shown to a stranger. The predicate is the authority
// (PRD 019 §8); the WHERE clause is an optimisation. This is where that ordering is made true rather
// than merely stated.
func (app *application) visibleGlimt(rows []glimt.Glimt, personID string, role users.Role) []glimt.Glimt {
	viewer := app.glimtViewer(personID, role)
	out := make([]glimt.Glimt, 0, len(rows))
	for _, g := range rows {
		if !users.MaySeeGlimt(viewer, glimtSubjectOf(g)) {
			app.Logger.Warn("glimt query returned a row the predicate refused",
				"glimtId", g.GlimtID, "audience", g.Audience, "userId", personID)
			continue
		}
		out = append(out, g)
	}
	return out
}

// pageParams reads limit/offset, leaving the clamping to the querier.
//
// Unparseable values are ignored rather than rejected: a client sending `limit=abc` gets the default
// page, which is more useful than a 400 for a parameter nothing important depends on.
func pageParams(r *http.Request) (limit, offset int) {
	q := r.URL.Query()
	if v, err := strconv.Atoi(q.Get("limit")); err == nil {
		limit = v
	}
	if v, err := strconv.Atoi(q.Get("offset")); err == nil {
		offset = v
	}
	return limit, offset
}

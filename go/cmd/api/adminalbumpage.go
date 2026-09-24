package main

import (
	"net/http"
	"net/url"

	"github.com/julienschmidt/httprouter"

	"nathejk.dk/nathejk/table/album"
)

// The album editor's page (PRD 022 §7, task 378).
//
// # Addressed by slug, like the public page
//
// Deliberately, so a curator looking at `/2026/album/natten` can reach `/2026/album/natten/edit` by editing the URL,
// which is how somebody actually navigates between the two while checking their work.
//
// The task notes the thing to be careful about: a draft's slug is guessable from the public side **only if the
// public read leaks**, and it does not — `album.Queries.BySlug` answers an identical "not found" for unknown,
// unpublished and deleted, which is what stops the open web enumerating drafts. This page reads through
// `album.CuratorQueries` instead, so it can see exactly what the public read refuses to.
//
// # What is editable here, and what is not
//
// Title, description, sort order and publication on the card; captions, and everything else the library can do,
// through the shared action bar over the album's photographs (task 396). **Not the slug** —
// it is the album's public address and a retitled album answering 404 is a dead link in a family's chat history.
// Neither the request shape nor `album.Updated` has a field for it.

// adminAlbumPageData is the album view's editor card.
//
// The photographs are not here: since task 396 they are the shared contact sheet, narrowed with `album={id}`, so
// the album view has the library's selection and actions rather than a list of its own.
type adminAlbumPageData struct {
	Root string

	AlbumID     string
	Slug        string
	Title       string
	Description string
	SortOrder   int
	Published   bool
	Deleted     bool

	// ItemCount is the album's live items — neither removed nor deleted from the library — which is what the
	// grid below shows.
	ItemCount int

	// PublicPath is where the album lives on the open web, shown so a curator can check their work. Rendered even
	// when unpublished, with the page saying it is not live yet: knowing the address a draft *will* have is
	// useful, and it is not a secret from somebody holding the admin credential.
	PublicPath string
}

// adminAlbumPageHandler serves one album's view: the editor card, then its photographs as the contact sheet.
//
// No OpenAPI annotations: an HTML page, like the tool's other two.
func (app *application) adminAlbumPageHandler(w http.ResponseWriter, r *http.Request) {
	if app.models.AlbumCurator == nil {
		app.ServiceUnavailableResponse(w, r, "albummerne er ikke tilgængelige lige nu")
		return
	}

	slug := httprouter.ParamsFromContext(r.Context()).ByName("slug")

	// The curator read is by **id**, because that is an album's identity, while the URL carries the slug — its
	// address. So the slug is resolved through the year's albums rather than by a second read: a handful of rows,
	// and it keeps `CuratorQueries` from growing a by-slug method that would duplicate the public one's shape
	// without its filtering.
	albums, err := app.models.AlbumCurator.All(app.config.eventYear)
	if err != nil {
		app.ServerErrorResponse(w, r, err)
		return
	}
	var found *album.CuratorAlbum
	for i := range albums {
		if albums[i].Slug == slug {
			found = &albums[i]
			break
		}
	}
	if found == nil {
		app.NotFoundResponse(w, r)
		return
	}
	a := *found

	app.renderAdminPage(w, adminPageData{
		View: "album",
		// The grid's first request, and "select all matching", both narrow to this album.
		Query: "album=" + url.QueryEscape(a.ID),
		Album: &adminAlbumPageData{
			Root:        app.publicRoot(),
			AlbumID:     a.ID,
			Slug:        a.Slug,
			Title:       a.Title,
			Description: a.Description,
			SortOrder:   a.SortOrder,
			Published:   a.Published,
			Deleted:     a.Deleted,
			ItemCount:   a.ItemCount,
			PublicPath:  app.publicRoot() + "/album/" + a.Slug,
		},
	})
}

package main

import (
	"html/template"
	"net/http"

	"github.com/julienschmidt/httprouter"
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
// Title, description, sort order, publication, the item order, and each photograph's caption. **Not the slug** —
// it is the album's public address and a retitled album answering 404 is a dead link in a family's chat history.
// Neither the request shape nor `album.Updated` has a field for it.

// adminAlbumPageData is what the editor renders.
type adminAlbumPageData struct {
	Year string

	AlbumID     string
	Slug        string
	Title       string
	Description string
	SortOrder   int
	Published   bool
	Deleted     bool

	// Items are the album's positions in order, removed ones included — which is the difference from the public
	// page, and the reason this reads through the curator interface.
	Items []adminAlbumItemView

	// PublicPath is where the album lives on the open web, shown so a curator can check their work. Rendered even
	// when unpublished, with the page saying it is not live yet: knowing the address a draft *will* have is
	// useful, and it is not a secret from somebody holding the admin credential.
	PublicPath string
}

// adminAlbumItemView is one position in the editor.
type adminAlbumItemView struct {
	Ordinal int
	PhotoID string
	Caption string

	// Removed is whether this position was taken out of the album.
	Removed bool
	// PhotoDeleted is whether the photograph itself was deleted from the library.
	//
	// Distinct from Removed, and the page says which: a removed item left *this album*, a deleted photograph left
	// *everywhere*, and only one of the two is fixed by re-adding it here. PRD 022 §5 requires the copy to make
	// that clear, and this is where it has to.
	PhotoDeleted bool

	// IsCover is whether this is the album's cover — the first live item, since there is no cover column.
	IsCover bool
}

// adminAlbumPageHandler serves one album's editor.
//
// No OpenAPI annotations: an HTML page, like `/admin` itself.
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
	var albumID string
	for _, a := range albums {
		if a.Slug == slug {
			albumID = a.ID
			break
		}
	}
	if albumID == "" {
		app.NotFoundResponse(w, r)
		return
	}

	a, items, found, err := app.models.AlbumCurator.Album(app.config.eventYear, albumID)
	if err != nil {
		app.ServerErrorResponse(w, r, err)
		return
	}
	if !found {
		app.NotFoundResponse(w, r)
		return
	}

	data := adminAlbumPageData{
		Year:        app.config.eventYear,
		AlbumID:     a.ID,
		Slug:        a.Slug,
		Title:       a.Title,
		Description: a.Description,
		SortOrder:   a.SortOrder,
		Published:   a.Published,
		Deleted:     a.Deleted,
		PublicPath:  "/" + app.config.eventYear + "/album/" + a.Slug,
		Items:       make([]adminAlbumItemView, 0, len(items)),
	}

	// The cover is the first **live** item, which is the definition the public read uses — there is no cover
	// column, so reordering is how a cover is chosen. Computed here rather than in the template, because a
	// template deciding it would be a second definition.
	coverSet := false
	for _, it := range items {
		view := adminAlbumItemView{
			Ordinal:      it.Ordinal,
			PhotoID:      it.PhotoID,
			Caption:      it.Caption,
			Removed:      it.Removed,
			PhotoDeleted: it.PhotoDeleted,
		}
		if !coverSet && !it.Removed && !it.PhotoDeleted {
			view.IsCover = true
			coverSet = true
		}
		data.Items = append(data.Items, view)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := adminAlbumTemplates.ExecuteTemplate(w, "adminalbum", data); err != nil {
		app.Logger.Error("rendering the admin album page", "err", err)
	}
}

// adminAlbumTemplates holds the editor's markup.
//
// Its own template rather than a second definition inside `adminTemplates`: the two pages share a look but not a
// layout, and one template set holding both would make every change to either a change to a file nobody can read
// in one screen. The styling is duplicated deliberately and is small; if a third admin page appears, that is the
// moment to extract a shared head.
// adminAlbumTemplates holds the editor's markup, assembled from adminui/album.{html,css,js} (task 394).
//
// Same arrangement as the index page's, and the same reason: a backtick anywhere in the CSS, JS or HTML used to
// terminate the Go raw string this lived in. The @inject markers are replaced with the other two files' source
// before parsing, so html/template still sees one document and still contextually escapes the markup's actions.
var adminAlbumTemplates = template.Must(template.New("adminalbum").Parse(
	mustInjectAdminAssets("adminui/album.html", "adminui/album.css", "adminui/album.js"),
))

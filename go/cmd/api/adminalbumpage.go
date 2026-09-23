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
// Deliberately, so a curator looking at `/2026/album/natten` can reach `/admin/album/natten` by editing the URL,
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
var adminAlbumTemplates = template.Must(template.New("adminalbum").Parse(`
{{define "adminalbum"}}<!doctype html>
<html lang="da">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="robots" content="noindex, nofollow">
<title>{{.Title}} — Billedarkiv {{.Year}}</title>
<style>
:root { color-scheme: light dark; }
* { box-sizing: border-box; }
body { margin: 0; font: 16px/1.5 -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
       background: #f4f4f5; color: #18181b; }
header { display: flex; align-items: baseline; gap: 1rem; flex-wrap: wrap;
         padding: 1rem 1.5rem; background: #18181b; color: #fafafa; }
h1 { margin: 0; font-family: Impact, Haettenschweiler, "Arial Narrow Bold", sans-serif;
     font-size: 1.5rem; font-weight: normal; letter-spacing: 0.02em; }
header a { color: #fbbf24; }
main { padding: 1.5rem; max-width: 60rem; }
h2 { font-family: Impact, Haettenschweiler, "Arial Narrow Bold", sans-serif;
     font-weight: normal; font-size: 1.125rem; letter-spacing: 0.02em; margin: 1.5rem 0 0.5rem; }
.card { background: #fff; border: 1px solid #e4e4e7; border-radius: 0.5rem; padding: 1rem; max-width: 40rem; }
label { display: block; font-size: 0.875rem; color: #52525b; margin-top: 0.75rem; }
input[type=text], textarea, input[type=number] {
  font: inherit; width: 100%; padding: 0.375rem 0.5rem;
  border: 1px solid #d4d4d8; border-radius: 0.375rem;
}
textarea { min-height: 4rem; }
button { font: inherit; font-size: 0.875rem; cursor: pointer;
         background: #18181b; color: #fafafa; border: 1px solid #18181b;
         padding: 0.375rem 0.75rem; border-radius: 0.375rem; }
button.ghost { background: #fff; color: #18181b; border-color: #d4d4d8; }
button:focus-visible, input:focus-visible, textarea:focus-visible { outline: 3px solid #2563eb; outline-offset: 2px; }
.state { display: inline-block; font-size: 0.8125rem; padding: 0.0625rem 0.5rem; border-radius: 999px; }
.state.live { background: #dcfce7; color: #166534; }
.state.draft { background: #fef3c7; color: #92400e; }
.state.gone { background: #fee2e2; color: #991b1b; }
#note { min-height: 1.5rem; color: #166534; }
.items { list-style: none; padding: 0; margin: 0.5rem 0 0; display: grid; gap: 0.5rem; }
.items li { display: grid; grid-template-columns: 5rem 1fr auto; gap: 0.75rem; align-items: start;
            background: #fff; border: 1px solid #e4e4e7; border-radius: 0.375rem; padding: 0.5rem; }
.items img { width: 5rem; height: 3.75rem; object-fit: cover; border-radius: 0.25rem; background: #e4e4e7; }
.items .noimg { width: 5rem; height: 3.75rem; border-radius: 0.25rem; background: #e4e4e7;
                display: grid; place-items: center; font-size: 0.75rem; color: #52525b; text-align: center; }
.items li.removed { opacity: 0.6; }
.items .meta { font-size: 0.8125rem; color: #52525b; }
.move { display: flex; flex-direction: column; gap: 0.25rem; }
.hint { color: #52525b; font-size: 0.875rem; }
</style>
</head>
<body>
<header>
  <h1>{{.Title}}</h1>
  <a href="/admin">← Alle billeder</a>
  {{if .Deleted}}<span class="state gone">slettet</span>
  {{else if .Published}}<span class="state live">udgivet</span>
  {{else}}<span class="state draft">kladde</span>{{end}}
</header>
<main data-album="{{.AlbumID}}">
  <p id="note" aria-live="polite"></p>

  <h2>Albummets oplysninger</h2>
  <div class="card">
    <label for="title">Titel</label>
    <input type="text" id="title" value="{{.Title}}" maxlength="120">

    <label for="desc">Beskrivelse</label>
    <textarea id="desc" maxlength="2000">{{.Description}}</textarea>

    <label for="sort">R&aelig;kkef&oslash;lge p&aring; forsiden</label>
    <input type="number" id="sort" value="{{.SortOrder}}" step="10">

    <!-- The slug is shown but not editable, and the page says why. A field a curator cannot change is worth
         explaining, or it reads as a bug. -->
    <label for="slug">Adresse</label>
    <input type="text" id="slug" value="{{.Slug}}" readonly>
    <p class="hint">Adressen kan ikke &aelig;ndres. Den er <code>{{.PublicPath}}</code>, og et album der skifter
    adresse bliver et d&oslash;dt link i de beskeder folk allerede har sendt.
    {{if .Published}}<a href="{{.PublicPath}}">Se den offentlige side</a>.{{else}}Den er ikke udgivet endnu.{{end}}</p>

    <p>
      <button type="button" id="save">Gem &aelig;ndringer</button>
      {{if .Published}}
        <button type="button" class="ghost" id="unpublish">Fjern fra forsiden</button>
      {{else}}
        <button type="button" id="publish">Udgiv p&aring; forsiden</button>
      {{end}}
    </p>
    <p class="hint">Udgivelse sl&aring;r igennem p&aring; forsiden inden for et minut.</p>
  </div>

  <h2>Billeder i albummet</h2>
  {{if not .Items}}
    <p class="hint">Der er ingen billeder i albummet endnu. V&aelig;lg dem p&aring;
    <a href="/admin">kontaktarket</a>.</p>
  {{else}}
  <p class="hint">Det f&oslash;rste billede er forsidebilledet. Billedteksten h&oslash;rer til billedet, s&aring;
  den er den samme i alle album billedet ligger i.</p>
  <ul class="items" id="items">
    {{range .Items}}
    <li data-photo="{{.PhotoID}}"{{if or .Removed .PhotoDeleted}} class="removed"{{end}}>
      {{if .PhotoDeleted}}
        <span class="noimg">slettet fra arkivet</span>
      {{else}}
        <img src="/api/admin/photos/{{.PhotoID}}/media?variant=thumb" alt="" loading="lazy" decoding="async">
      {{end}}
      <div>
        <label for="cap-{{.Ordinal}}">Billedtekst</label>
        <input type="text" id="cap-{{.Ordinal}}" class="cap" value="{{.Caption}}" maxlength="1000"
               data-photo="{{.PhotoID}}"{{if .PhotoDeleted}} disabled{{end}}>
        <p class="meta">
          Plads {{.Ordinal}}{{if .IsCover}} &middot; forsidebillede{{end}}
          <!-- The two removals read differently, because only one of them is fixed by re-adding the
               photograph here. PRD 022 §5. -->
          {{if .PhotoDeleted}} &middot; billedet er slettet fra arkivet, s&aring; det kan ikke vises igen herfra
          {{else if .Removed}} &middot; taget ud af dette album &mdash; l&aelig;g det tilbage fra kontaktarket{{end}}
        </p>
      </div>
      <div class="move">
        {{if not (or .Removed .PhotoDeleted)}}
        <button type="button" class="ghost up" aria-label="Flyt op">&uarr;</button>
        <button type="button" class="ghost down" aria-label="Flyt ned">&darr;</button>
        {{end}}
      </div>
    </li>
    {{end}}
  </ul>
  <p><button type="button" id="saveorder">Gem r&aelig;kkef&oslash;lge</button></p>
  {{end}}
</main>
<script>
// The album editor (task 378).
//
// Ordinals are moved with buttons rather than dragged: PRD 022 §4 puts drag-to-reorder explicitly outside the
// launch requirements, and two buttons are keyboard-operable for free, which a drag handle is not.
(() => {
  'use strict';

  const main = document.querySelector('main');
  const albumId = main.dataset.album;
  const note = document.getElementById('note');
  const items = document.getElementById('items');

  function say(text, bad) {
    note.textContent = text;
    note.style.color = bad ? '#b91c1c' : '#166534';
  }

  async function send(path, method, body) {
    const res = await fetch(path, {
      method,
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });
    if (res.status === 204) return {};
    const out = await res.json().catch(() => null);
    if (!res.ok) throw new Error((out && out.error) || 'Fejl ' + res.status);
    return out || {};
  }

  // --- the album's own fields ----------------------------------------------

  document.getElementById('save').addEventListener('click', async () => {
    say('Gemmer…');
    try {
      // Only the fields on this form are sent. The request's pointer semantics mean anything omitted is left
      // alone, so this cannot disturb the publication state — which has its own buttons for exactly that reason.
      await send('/api/admin/albums/' + encodeURIComponent(albumId), 'PATCH', {
        title: document.getElementById('title').value,
        description: document.getElementById('desc').value,
        sortOrder: parseInt(document.getElementById('sort').value, 10) || 0,
      });
      say('Gemt.');
    } catch (err) {
      say(err.message, true);
    }
  });

  for (const [id, published] of [['publish', true], ['unpublish', false]]) {
    const b = document.getElementById(id);
    if (!b) continue;
    b.addEventListener('click', async () => {
      say(published ? 'Udgiver…' : 'Fjerner…');
      try {
        // Publication is sent **alone**, so an unsaved edit in the form above cannot ride along with it. A
        // curator pressing "udgiv" has said one thing and should not accidentally publish a half-typed title.
        await send('/api/admin/albums/' + encodeURIComponent(albumId), 'PATCH', { published });
        // Reloaded rather than patched in place: the header's state, the buttons and the public link all change
        // together, and re-rendering them by hand is three chances to show a stale one.
        location.reload();
      } catch (err) {
        say(err.message, true);
      }
    });
  }

  // --- captions -------------------------------------------------------------
  //
  // Saved on blur rather than with a button, because a caption per photograph would otherwise mean a button per
  // photograph. The caption belongs to the **photograph**, so this changes it in every album it appears in —
  // which the page says above the list, because it would otherwise be a surprise.

  if (items) {
    for (const input of items.querySelectorAll('.cap')) {
      let original = input.value;
      input.addEventListener('blur', async () => {
        if (input.value === original) return;
        try {
          await send('/api/admin/photos', 'PATCH', {
            photoIds: [input.dataset.photo],
            caption: input.value,
          });
          original = input.value;
          say('Billedtekst gemt.');
        } catch (err) {
          say(err.message, true);
        }
      });
    }

    // --- the order ----------------------------------------------------------

    items.addEventListener('click', (e) => {
      const up = e.target.closest('.up');
      const down = e.target.closest('.down');
      if (!up && !down) return;

      const li = e.target.closest('li');
      // Only live items move. A removed one has no position in the order being sent, and swapping past it would
      // silently change what "next" means.
      const movable = Array.from(items.querySelectorAll('li:not(.removed)'));
      const i = movable.indexOf(li);
      if (i < 0) return;

      const j = up ? i - 1 : i + 1;
      if (j < 0 || j >= movable.length) return;

      // Moved in the DOM only. Nothing is saved until "Gem rækkefølge", so a curator can shuffle freely and
      // change their mind without a dozen events on the log.
      if (up) items.insertBefore(li, movable[j]);
      else items.insertBefore(movable[j], li);
      say('Rækkefølgen er ikke gemt endnu.', true);
    });

    document.getElementById('saveorder').addEventListener('click', async () => {
      const order = Array.from(items.querySelectorAll('li:not(.removed)')).map((li) => li.dataset.photo);
      if (!order.length) { say('Der er ingen billeder at sortere.', true); return; }

      say('Gemmer rækkefølge…');
      try {
        // The whole live order in one request. The server publishes one event for it, because moving one item
        // into a position another holds is not expressible as independent writes — see album's ItemsReordered.
        await send('/api/admin/albums/' + encodeURIComponent(albumId) + '/items', 'PATCH', { photoIds: order });
        location.reload();
      } catch (err) {
        say(err.message, true);
      }
    });
  }
})();
</script>
</body>
</html>{{end}}
`))

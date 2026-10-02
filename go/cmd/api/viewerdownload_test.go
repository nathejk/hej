package main

import (
	"strings"
	"testing"
)

// Saving the photograph on screen (task 488).
//
// # Why these are source-reading tests
//
// Nothing here can execute the viewer's JavaScript — the constraint every test in `viewer_test.go` works under and
// states at length. So the properties worth pinning are the ones a reader of a diff would not notice going wrong, and
// for this control there are exactly two:
//
//   - **which bytes it saves**, because saving the wrong ones is a privacy failure rather than a bug;
//   - **that it stays a read**, because this file is one copy shared by the public album page and the curator's tool.

// The download saves the display image, and the viewer has no way to ask for anything bigger.
//
// The display image is already stripped of metadata by the ingest pipeline, which is what makes it safe to hand to
// anybody who can already see it. The photographer's file is not, and is reachable only behind the curator's
// credential (PRD 027 R10/R11) — `originalboundary_test.go` is what holds that, and this is the same rule stated from
// the other side: here is the control that would be the obvious place to break it.
func TestTheDownloadSavesTheDisplayImage(t *testing.T) {
	code := withoutComments(viewerAsset(t, "viewer.js"))

	reg := strings.Index(code, "register('download'")
	if reg < 0 {
		t.Fatal("viewer.js registers no download control")
	}
	body := code[reg:]
	if end := strings.Index(body, "\n  });"); end > 0 {
		body = body[:end]
	}

	// `item.full` is the display image the <img> already shows. Asserted as the assignment rather than as a
	// substring: `a.href = ` is the one line that decides what gets saved.
	if !strings.Contains(body, "a.href = ctx.item.full") {
		t.Errorf("the download must save ctx.item.full, the display image already on screen.\ngot: %s", body)
	}
	// And not the smaller renditions, which would hand somebody a thumbnail when they asked for the photograph.
	for _, wrong := range []string{"ctx.item.thumb", "ctx.item.medium"} {
		if strings.Contains(body, wrong) {
			t.Errorf("the download reaches for %s; \"a fair size version\" is the display image, and a visitor who "+
				"wanted the thumbnail would not have pressed save", wrong)
		}
	}
}

// The download is a read, and it is the control most likely to be turned into a write by accident.
//
// Not because somebody would add a POST to it, but because "let the curator download the file" is a one-word change in
// a file that both surfaces load. The guard is the same shape as the public page's action allowlist: name what is
// allowed rather than hunt for what is not.
func TestTheDownloadControlWritesNothing(t *testing.T) {
	code := withoutComments(viewerAsset(t, "viewer.js"))

	reg := strings.Index(code, "register('download'")
	if reg < 0 {
		t.Fatal("viewer.js registers no download control")
	}
	body := code[reg:]
	if end := strings.Index(body, "\n  });"); end > 0 {
		body = body[:end]
	}

	for _, forbidden := range []string{"fetch(", "XMLHttpRequest", "method:", "FormData", "navigator.share"} {
		if strings.Contains(body, forbidden) {
			t.Errorf("the download control mentions %s. It is an <a download> and nothing more: the browser "+
				"reuses the cached response the <img> already has, and anything that issues its own request is "+
				"either re-downloading bytes on a phone or doing something other than saving.", forbidden)
		}
	}
}

// The anchor is in the document when it is clicked.
//
// Firefox ignores a click on a disconnected anchor, so this is the difference between the button working on three
// engines and on four. It is also exactly the kind of line somebody tidies away as redundant — `display:none` looks
// like it should be enough, and it is not: the element has to be *connected*, not invisible.
func TestTheDownloadAnchorIsConnectedBeforeItIsClicked(t *testing.T) {
	code := withoutComments(viewerAsset(t, "viewer.js"))

	appendAt := strings.Index(code, "document.body.appendChild(a)")
	clickAt := strings.Index(code, "a.click()")
	removeAt := strings.Index(code, "document.body.removeChild(a)")

	if appendAt < 0 || clickAt < 0 || removeAt < 0 {
		t.Fatal("the download control no longer appends, clicks and removes an anchor; this guard needs updating")
	}
	if !(appendAt < clickAt && clickAt < removeAt) {
		t.Error("the anchor must be appended, then clicked, then removed. Firefox ignores a click on an anchor " +
			"that is not in the document, so the order is the behaviour rather than tidiness.")
	}
}

// The filename is derived from the album, and never from the photographer's filename.
//
// `data-filename` is the name the photographer's file had (task 475). It is **admin-only** — the public album page does
// not emit it, and `TestThePublicAlbumPageCarriesNoFilename` holds that — so a download that used it would produce a
// filename on the public surface that is blank there and meaningful in the curator's tool, which is the worst of both:
// it would work in testing and be empty in production.
func TestTheDownloadFilenameIgnoresThePhotographersFilename(t *testing.T) {
	code := withoutComments(viewerAsset(t, "viewer.js"))

	start := strings.Index(code, "function downloadName(")
	if start < 0 {
		t.Fatal("viewer.js has no downloadName; this guard needs updating")
	}
	body := code[start:]
	if end := strings.Index(body[1:], "\n  function "); end > 0 {
		body = body[:end]
	}

	if strings.Contains(body, "item.filename") {
		t.Error("the download filename uses item.filename, which is the photographer's own filename and exists " +
			"only behind the curator's credential (task 475). On the public page it is empty, so this would be a " +
			"name that works in the admin tool and vanishes in production.")
	}
	// It is derived from the permalink, which is the only string in the viewer that knows which album this is — the
	// viewer itself holds no album concept (PRD 023 §7.7) and must not gain one.
	if !strings.Contains(body, "ctx.item.permalink") {
		t.Error("the download filename should come from the permalink's album segment; the viewer knows nothing " +
			"else about which album it is showing")
	}
	// With a fallback, because a host page is not obliged to supply a permalink and a download with no filename is
	// a file called "media" in somebody's downloads folder.
	if !strings.Contains(body, "ctx.config.shareTitle") {
		t.Error("with no permalink the filename should fall back to the album title, or the saved file is named " +
			"after the route rather than after the album")
	}
}

// The curator's tool does not declare this control, and that is a decision rather than an omission.
//
// It has the whole album as a zip at four sizes and the photographer's file besides (PRD 027), so a button that saves
// the 1600px rendition would be the least useful of the five ways out of that page. Asserted because the opposite edit
// — adding it "for consistency" — is the one that would put a save button next to a caption editor and invite somebody
// to wonder which of the two surfaces they are on.
func TestTheCuratorsViewerDoesNotDeclareTheDownload(t *testing.T) {
	src := adminPageSource(t)

	for _, declared := range actionDeclarations(src) {
		for _, name := range strings.Split(declared, ",") {
			if strings.TrimSpace(name) == "download" {
				t.Error("the admin tool declares the viewer's download control. It already offers the album as a " +
					"zip at four sizes and the photographer's own file, so this would be the least useful way out " +
					"of that page — and PRD 027's point is that a curator wants the original, not the rendition.")
			}
		}
	}
}

// actionDeclarations returns every `data-viewer-actions` value in a page.
func actionDeclarations(src string) []string {
	var out []string
	rest := src
	const attr = `data-viewer-actions="`
	for {
		i := strings.Index(rest, attr)
		if i < 0 {
			return out
		}
		rest = rest[i+len(attr):]
		j := strings.Index(rest, `"`)
		if j < 0 {
			return out
		}
		out = append(out, rest[:j])
		rest = rest[j:]
	}
}

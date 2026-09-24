package main

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"nathejk.dk/nathejk/table/photo"
)

// Danish counts (task 387).
//
// # What went wrong, and why it is not a typo
//
// The frontpage's album card read `{{.Count}} billeder`, so an album holding one photograph said **"1 billeder"**
// on the one surface families read.
//
// It was not a typo. The same singular/plural choice was written out by hand at **fourteen** call sites — six in
// Go, eight in JavaScript — and the frontpage was the fifteenth, written by somebody who had not happened to see
// any of them. That is the shape of this class of bug: not one careless line, but a grammatical fact with no home,
// so each new caller re-derives it and one of them eventually does not.
//
// So the tests below are about the home, not about the line.

func TestPhotoCountHasASingular(t *testing.T) {
	for n, want := range map[int]string{
		0:   "0 billeder",
		1:   "1 billede",
		2:   "2 billeder",
		312: "312 billeder",
	} {
		if got := photoCount(n); got != want {
			t.Errorf("photoCount(%d) = %q, want %q", n, got, want)
		}
	}
}

// **`album` does not inflect in Danish**, and this asserts it stays that way.
//
// The trap is the analogy: `billede` → `billeder` makes `album` → `albummer` look right, and it is not. A test
// that only checked the photograph count would leave the next person to add an album count free to guess.
func TestAlbumCountDoesNotInflect(t *testing.T) {
	for n, want := range map[int]string{0: "0 album", 1: "1 album", 7: "7 album"} {
		if got := albumCount(n); got != want {
			t.Errorf("albumCount(%d) = %q, want %q", n, got, want)
		}
	}
}

// **No caller writes the plural out by hand any more.**
//
// This is the test that actually closes task 387. Fixing the frontpage alone would have left thirteen copies of the
// rule in place and the fourteenth free to be written wrong — and the task's own note said a fix that leaves
// siblings wrong is worse than none, because it makes the remaining ones look deliberate.
//
// Asserted against the sources rather than through behaviour, because the property is "there is one definition",
// which no response can show. Comments are stripped first: the prose in plural.go quotes the very pattern it
// replaced, and a needle matching the comment that explains a rule rather than the code implementing it is a
// mistake this package has made three times.
func TestNothingWritesTheDanishPluralByHand(t *testing.T) {
	// The two shapes that were in the codebase: a Go `billeder := "billeder"` idiom, and a JavaScript ternary
	// picking between ' billede' and ' billeder'.
	hand := []struct {
		pattern *regexp.Regexp
		why     string
	}{
		{regexp.MustCompile(`billeder\s*:?=\s*"billeder"`),
			"use photoCount(n) from plural.go"},
		{regexp.MustCompile(`"billede"`),
			"a bare singular noun means somebody is assembling the count by hand; use photoCount(n)"},
		{regexp.MustCompile(`'1 billede'`),
			"use ctx.photoCount(n)"},
		{regexp.MustCompile(`' billeder'`),
			"use ctx.photoCount(n)"},
		{regexp.MustCompile(`\{\{\s*\.\w*Count\s*\}\}\s+billeder`),
			"use the `photos` template function"},
		{regexp.MustCompile(`\{\{\s*\.\w*Count\s*\}\}\s+album`),
			"use the `albums` template function"},
	}

	// Every file that renders a count: the Go sources on these surfaces, the page's scripts, and the templates.
	files := []string{
		"adminalbum.go", "adminpatrol.go", "adminposition.go", "adminlibrary.go", "adminfragments.go",
		"publicsite.go", "albumpage.go", "glimtpublic.go",
	}
	assets := []string{"adminui/fragments.html", "adminui/page.html"}
	assets = append(assets, adminPageScripts...)

	// The two places a hand-written plural is correct, each with its reason. An allowlist rather than a looser
	// pattern, because "the definition" and "a demonstrative" are the only two cases and loosening the pattern to
	// admit them would also admit the bug.
	//
	// Every entry is checked to still match something, so an exemption cannot outlive the code it excuses.
	exempt := map[string]struct{ text, why string }{
		"adminui/main.js": {
			text: "ctx.photoCount = (n) => (n === 1 ? '1 billede' : n + ' billeder');",
			why:  "this is the definition; TestTheAdminCountsAgreeAcrossGoAndJavaScript holds it to the Go one",
		},
		"adminui/deleteaction.js": {
			text: "const word = n === 1 ? 'dette billede' : 'disse ' + n + ' billeder';",
			why: "the delete confirmation asks about *these* photographs, and a demonstrative does not compose " +
				"with a count: \"slet 1 billede?\" is a form, \"slet dette billede?\" is a question (task 379)",
		},
	}

	for _, name := range files {
		src := stripGoComments(adminSource(t, name))
		for _, h := range hand {
			if m := h.pattern.FindString(src); m != "" {
				t.Errorf("%s writes a Danish plural by hand (%q): %s", name, m, h.why)
			}
		}
	}
	for _, name := range assets {
		src := stripJSLineComments(stripTemplateComments(mustReadAdminAsset(name)))
		if e, ok := exempt[name]; ok {
			if !strings.Contains(src, e.text) {
				t.Errorf("%s is exempted for %q, which is no longer there. Either the exemption is stale and "+
					"should go, or the line moved and the reason needs revisiting: %s", name, e.text, e.why)
				continue
			}
			src = strings.Replace(src, e.text, "", 1)
		}
		for _, h := range hand {
			if m := h.pattern.FindString(src); m != "" {
				t.Errorf("%s writes a Danish plural by hand (%q): %s", name, m, h.why)
			}
		}
	}
}

// **The Go and JavaScript definitions agree.**
//
// There are two copies and there is no way to have fewer: this surface has no build step, so nothing compiles
// `plural.go` and `main.js` together. Two is the floor, and an unwatched floor is how the curator's sheet comes to
// say one thing while the frontpage says another about the same album.
//
// So the JavaScript is exercised the only way it can be from here — by reading the definition out of main.js and
// checking it against the Go one for the cases that matter. A crude comparison, and it catches the whole of what
// can realistically drift: the singular threshold and the two words.
func TestTheAdminCountsAgreeAcrossGoAndJavaScript(t *testing.T) {
	js := stripJSLineComments(mustReadAdminAsset("adminui/main.js"))

	line := regexp.MustCompile(`ctx\.photoCount\s*=\s*\(n\)\s*=>\s*\(n === (\d+) \? '([^']+)' : n \+ '([^']+)'\)`)
	m := line.FindStringSubmatch(js)
	if m == nil {
		t.Fatalf("could not find ctx.photoCount's definition in main.js; if it changed shape, this test has to " +
			"change with it rather than be deleted — two copies of a grammatical rule need something watching them")
	}
	threshold, singular, pluralSuffix := m[1], m[2], m[3]

	if threshold != "1" {
		t.Errorf("JavaScript takes the singular at n === %s, Go takes it at 1", threshold)
	}
	if singular != photoCount(1) {
		t.Errorf("JavaScript says %q for one photograph, Go says %q", singular, photoCount(1))
	}
	// Go renders "12 billeder"; the JavaScript appends its suffix to the number, so the suffix has to be what Go
	// produces minus the number.
	if want := strings.TrimPrefix(photoCount(12), "12"); pluralSuffix != want {
		t.Errorf("JavaScript appends %q, Go produces %q", pluralSuffix, want)
	}
}

// **Both template sets can call the counts.**
//
// The public site and the admin tool are separate `template.Template` values with separate function maps, so
// registering a function on one does not register it on the other — and a template calling a function its set does
// not have fails at *parse* time, which for both of these is `init`. That makes the failure a binary that refuses
// to start rather than a page that breaks under a curator, so this test is mostly about saying which functions are
// expected to be there.
func TestBothTemplateSetsCanCountInDanish(t *testing.T) {
	// Both sets parsed at all, which is what proves the functions resolved: a template calling a function its set
	// does not have is a parse error, and these parse at init.
	for _, tc := range []struct {
		defined string
		name    string
	}{
		{publicSiteTemplates.DefinedTemplates(), "the public site"},
		{adminTemplates.DefinedTemplates(), "the admin tool"},
	} {
		if tc.defined == "" {
			t.Errorf("%s has no templates at all", tc.name)
		}
	}

	// The registrations themselves, since the above only proves the sets parsed.
	if _, ok := publicSiteFuncs["photos"]; !ok {
		t.Error("the public site cannot count photographs; its frontpage card needs `photos`")
	}
	for _, fn := range []string{"photos", "albums"} {
		if _, ok := adminTemplateFuncs[fn]; !ok {
			t.Errorf("the admin tool's templates cannot call %q", fn)
		}
	}

	// And they are the same function, not two definitions that happen to agree today.
	if fmt.Sprintf("%p", publicSiteFuncs["photos"]) != fmt.Sprintf("%p", adminTemplateFuncs["photos"]) {
		t.Error("the two surfaces register different `photos` functions: one album's count would eventually read " +
			"differently on the frontpage than in the curator's list")
	}
}

// stripGoComments removes `//` comments from Go source.
//
// Same naivety, and the same safe direction, as stripJSLineComments in admin_test.go: it over-strips a `//` inside
// a string literal, which can only make a guard stricter.
func stripGoComments(src string) string { return stripJSLineComments(src) }

// stripTemplateComments removes `{{/* … */}}` blocks.
//
// fragments.html documents each fragment in a template comment, and several of those comments quote the counts they
// render — so without this, a guard here would match the prose rather than the markup.
func stripTemplateComments(src string) string {
	for {
		i := strings.Index(src, "{{/*")
		if i < 0 {
			return src
		}
		j := strings.Index(src[i:], "*/}}")
		if j < 0 {
			return src[:i]
		}
		src = src[:i] + src[i+j+len("*/}}"):]
	}
}

// **Agreement is not finished when the noun agrees.**
//
// After `photoCount` fixed the count, the position message still read "Position sat på 1 billede. *De* vises på
// kortet." — found by reading the live message for a one-photograph selection, not by looking for it. That is the
// second half of task 387 and the half that would have been left behind.
func TestPhotoPronounAgreesWithTheCount(t *testing.T) {
	if got := photoPronoun(1); got != "det" {
		t.Errorf("photoPronoun(1) = %q, want \"det\"", got)
	}
	for _, n := range []int{0, 2, 40} {
		if got := photoPronoun(n); got != "de" {
			t.Errorf("photoPronoun(%d) = %q, want \"de\"", n, got)
		}
	}
}

// Every position outcome reads correctly for **one** photograph, in all four verdicts.
//
// The four are not interchangeable and PRD 022 §7 asks for this copy to be written carefully: `outside` is a
// statement about the photograph, `unknown` a statement about us. So each is checked rather than one standing in
// for the rest — a singular fixed in three branches and missed in the fourth is the failure this class of bug
// keeps producing.
func TestThePositionMessageReadsCorrectlyForOnePhotograph(t *testing.T) {
	for _, tc := range []struct{ verdict, want string }{
		{photo.BoundsInside, "Position sat på 1 billede. Det vises på kortet."},
		{photo.BoundsOutside, "Position sat på 1 billede, men den ligger uden for løbsområdet, " +
			"så det vises ikke på kortet."},
		{photo.BoundsUnknown, "Position sat på 1 billede. Den kunne ikke vurderes, fordi ingen poster " +
			"har en placering endnu — så det vises ikke på kortet."},
		{photo.BoundsNone, "Position sat på 1 billede."},
	} {
		if got := adminVerdictMessage(1, tc.verdict); got != tc.want {
			t.Errorf("verdict %q for one photograph:\n  got  %q\n  want %q", tc.verdict, got, tc.want)
		}
	}

	// And the plural still reads as it did, since this is a change to the singular and not to the sentence.
	if got := adminVerdictMessage(12, photo.BoundsInside); got != "Position sat på 12 billeder. De vises på kortet." {
		t.Errorf("the plural changed: %q", got)
	}

	// `den` survives in both branches that have it. It refers to the **position**, which is singular whatever the
	// selection holds — so a future tidy-up that swept it into photoPronoun would be wrong, and this says so.
	if !strings.Contains(adminVerdictMessage(12, photo.BoundsOutside), "men den ligger uden for") {
		t.Error("`den` refers to the position, not to the photographs, and must not be pluralised with them")
	}
}

// The other three sentences a curator reads after a bulk action, for one photograph.
func TestTheBulkActionMessagesReadCorrectlyForOnePhotograph(t *testing.T) {
	if got := adminAddedMessage(1, 1); got != "1 billede lagt i albummet." {
		t.Errorf("adding one photograph to one album: %q", got)
	}
	if got := adminAddedMessage(1, 3); got != "1 billede lagt i 3 album." {
		t.Errorf("adding one photograph to three albums: %q", got)
	}
	// `album` does not inflect, so three albums and one album differ only in the number.
	if got := adminAddedMessage(12, 3); got != "12 billeder lagt i 3 album." {
		t.Errorf("adding twelve photographs to three albums: %q", got)
	}
}

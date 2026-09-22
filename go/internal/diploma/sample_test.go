package diploma

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A sample renderer, and a regression test for the bug it caught.

// TestWriteSample writes a diploma to the path in DIPLOMA_SAMPLE, for looking at.
//
//	DIPLOMA_SAMPLE=/tmp/diploma.pdf go test ./internal/diploma/ -run TestWriteSample
//
// # Why this is a test and not a cmd
//
// Because it is not a tool anybody runs in production — it exists so that whoever replaces the artwork
// (`ReplaceBeforeLaunch`) can **look at the result**, which is the only way to catch the two things that have
// already gone wrong here: text landing on top of the artwork's own headline, and mojibake from the core fonts'
// single-byte encoding. Both passed every unit test at the time.
//
// Skipped unless the variable is set, so it costs nothing in CI.
func TestWriteSample(t *testing.T) {
	path := os.Getenv("DIPLOMA_SAMPLE")
	if path == "" {
		t.Skip("set DIPLOMA_SAMPLE=/tmp/diploma.pdf to write a sample to look at")
	}

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("creating %s: %v", path, err)
	}
	defer f.Close()

	at := time.Date(2026, 9, 20, 3, 42, 0, 0, time.UTC)
	// A real photograph, because the photograph is the element a rendered sample exists to check: its box, its
	// aspect ratio, and whether the text still clears it. testdata/patrol.jpg is one of `diplom`'s own 2024
	// fixtures — a patrol at the start line, 2000×1500.
	photo, err := os.ReadFile(filepath.Join("testdata", "patrol.jpg"))
	if err != nil {
		t.Fatalf("reading the sample photograph: %v", err)
	}

	err = PDF(Diploma{
		Number:           "42",
		Name:             "Ørnene",
		Title:            "Nathejk 2026",
		Route:            "fra Lundby til Glumsø",
		FinishedAt:       &at,
		Photo:            photo,
		PhotoContentType: "image/jpeg",
	}, f)
	if err != nil {
		t.Fatalf("PDF: %v", err)
	}
	t.Logf("wrote %s — open it and read it", path)
}

// **Danish must survive the render.** fpdf's core fonts are single-byte, so handing them UTF-8 prints "Ãrnene"
// where "Ørnene" belongs. `diplom` re-encoded to Latin-1 for exactly this reason; I dropped that in the port and
// a rendered sample is what caught it.
func TestDanishCharactersAreEncodedForTheCoreFonts(t *testing.T) {
	got := latin1("Ørnene gik i mål lørdag — næsten")

	// Ø is a single 0xD8 **byte** in Latin-1. Checked as a byte, not a rune: `strings.ContainsRune` would look
	// for U+00D8 encoded as UTF-8 (0xC3 0x98), which is precisely what must *not* be there — a mistake worth
	// leaving a note about, since it fails in the direction that looks like a bug in the code.
	if strings.IndexByte(got, 0xD8) < 0 {
		t.Errorf("Ø was not encoded to Latin-1: % x", got)
	}
	if strings.Contains(got, "\xc3\x98") {
		t.Errorf("the string is still UTF-8, which prints as mojibake: % x", got)
	}
	// One byte per character, for a string that is all Latin-1.
	if len(got) != len([]rune("Ørnene gik i mål lørdag — næsten")) {
		t.Errorf("want one byte per character, got %d bytes for %d runes",
			len(got), len([]rune("Ørnene gik i mål lørdag — næsten")))
	}
}

// A character Latin-1 cannot express is replaced, not fatal. A patrol name we cannot spell is a blemish on one
// diploma; an error is no diploma at all.
func TestAnUnmappableCharacterDoesNotFailTheRender(t *testing.T) {
	got := latin1("Ørnene 🦅 Łódź")
	if got == "" {
		t.Fatal("want a best-effort string rather than nothing")
	}

	var buf strings.Builder
	if err := PDF(Diploma{Number: "1", Name: "Ørnene 🦅 Łódź", Title: "Nathejk 2026"}, &buf); err != nil {
		t.Errorf("a name with unmappable characters must still render: %v", err)
	}
}

// **Typographic punctuation is folded to its ASCII cousin rather than replaced.** Latin-1 has no em dash or
// curly quote, and the encoder's substitute is 0x1A — a *control* character, which a PDF viewer draws as a box
// or nothing at all. A patrol called “Rævene” would have printed with two boxes around it.
func TestTypographicPunctuationSurvivesAsAscii(t *testing.T) {
	got := latin1("“Rævene” — Lars’ hold … ja")

	if strings.IndexByte(got, 0x1A) >= 0 {
		t.Errorf("a control character reached the PDF: % x", got)
	}
	for _, want := range []string{`"R`, `" - Lars' hold ... ja`} {
		if !strings.Contains(got, want) {
			t.Errorf("want %q in %q", want, got)
		}
	}
}

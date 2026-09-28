package photo

import (
	"strings"
	"testing"
)

// NormalizeFileName (task 448, PRD 024 §6 R5).
//
// # Why this is the one function worth testing hard here
//
// It is the entire boundary between a foreign filesystem and a `VARCHAR(255)` in a projection. Everything on
// the other side of it — the sort, the column, the log — assumes the value is short, valid UTF-8, printable
// and not a path. Each of those assumptions has a way of being wrong that a real browser has produced.

func TestNormalizeFileNameTakesTheBasename(t *testing.T) {
	for name, tc := range map[string]struct{ in, want string }{
		// Some browsers send this for a file picked from disk. Storing it would store the lie as well as
		// somebody's directory layout.
		"the fakepath prefix": {`C:\fakepath\IMG_0001.JPG`, "IMG_0001.JPG"},
		// A folder drop carries a relative path — and the path is the part most likely to name a person:
		// "Billeder/Mormors 80 års/" is a directory somebody made, not a photograph's name.
		"a relative path":  {"Nathejk 2026/Post 3/IMG_0002.JPG", "IMG_0002.JPG"},
		"mixed separators": {`Billeder\2026/IMG_0003.JPG`, "IMG_0003.JPG"},
		"no path at all":   {"IMG_0004.JPG", "IMG_0004.JPG"},
		"a trailing slash": {"Billeder/", ""},
	} {
		if got := NormalizeFileName(tc.in); got != tc.want {
			t.Errorf("%s: NormalizeFileName(%q) = %q, want %q", name, tc.in, got, tc.want)
		}
	}
}

// A filename is bytes from a filesystem, not necessarily text. An invalid sequence stored in a utf8mb4 column
// renders as a replacement glyph everywhere afterwards, and a control character turns one log line into two.
func TestNormalizeFileNameDropsWhatIsNotText(t *testing.T) {
	for name, tc := range map[string]struct{ in, want string }{
		"invalid UTF-8":     {"IMG_\xff\xfe0005.JPG", "IMG_0005.JPG"},
		"a newline":         {"IMG_0006\n.JPG", "IMG_0006.JPG"},
		"a NUL":             {"IMG_0007\x00.JPG", "IMG_0007.JPG"},
		"a delete":          {"IMG_0008\x7f.JPG", "IMG_0008.JPG"},
		"surrounding space": {"  IMG_0009.JPG  ", "IMG_0009.JPG"},
		// Danish filenames are the normal case here and must survive intact.
		"Danish letters": {"Bålet på ængen.jpg", "Bålet på ængen.jpg"},
		"an emoji":       {"🔥 bålet.jpg", "🔥 bålet.jpg"},
	} {
		if got := NormalizeFileName(tc.in); got != tc.want {
			t.Errorf("%s: NormalizeFileName(%q) = %q, want %q", name, tc.in, got, tc.want)
		}
	}
}

// The cap is in **runes**, and never mid-rune.
//
// The column counts characters, so a byte-counted limit would both cut a Danish name a character early and,
// worse, could split a multi-byte rune — which MariaDB stores as an invalid sequence. Same reasoning as
// `maxCreditRunes`; this is the second field to need it, which is why `truncateRunes` is shared.
func TestNormalizeFileNameCapsWithoutSplittingARune(t *testing.T) {
	long := strings.Repeat("å", 400) + ".jpg"
	got := NormalizeFileName(long)

	if n := len([]rune(got)); n != maxFileNameRunes {
		t.Errorf("capped to %d runes, want %d", n, maxFileNameRunes)
	}
	if !strings.HasPrefix(long, got) {
		t.Error("the cap must be a prefix of the original, not a re-encoding of it")
	}
	for _, r := range got {
		if r == '\uFFFD' {
			t.Fatal("the cap split a rune: MariaDB would store an invalid sequence, and every reader " +
				"thereafter renders a replacement glyph")
		}
	}
	// A value already inside the cap is returned untouched, which is the overwhelmingly common case.
	if got := NormalizeFileName("IMG_0010.JPG"); got != "IMG_0010.JPG" {
		t.Errorf("an ordinary name must be returned as-is, got %q", got)
	}
}

// "" in, "" out. The raw-body upload path carries no filename, and that has to be an ordinary answer rather
// than something the fold has to guard against: it is what the whole `IF(VALUES(fileName)="", …)` clause in
// the upsert is built on.
func TestNormalizeFileNameAcceptsNoName(t *testing.T) {
	for _, in := range []string{"", "   ", "\x00", `C:\fakepath\`} {
		if got := NormalizeFileName(in); got != "" {
			t.Errorf("NormalizeFileName(%q) = %q, want an empty name", in, got)
		}
	}
}

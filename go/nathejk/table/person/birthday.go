package person

import "time"

// birthdayLocation is the timezone the upstream birthdays were recorded in.
//
// This is not a stylistic choice, it is a correctness one. See parseBirthday.
const birthdayLocation = "Europe/Copenhagen"

// birthdayFormats are the shapes a birthday actually arrives in.
//
// types.Date documents itself as "2006-01-02" and shared-go's projectors write it
// straight through — but the real event stream carries values like
// "2010-06-03T22:00:00.000Z". shared-go stores birthday as VARCHAR(99), so it never
// noticed; this projection uses a DATE column and MariaDB rejected 6529 statements on
// the first replay against real data. Every one of those dead letters was this.
var birthdayFormats = []string{
	"2006-01-02",
	time.RFC3339,
	"2006-01-02T15:04:05.000Z07:00",
	"2006-01-02T15:04:05Z07:00",
	"2006-01-02T15:04:05",
}

// parseBirthday converts an upstream birthday into a DATE literal (YYYY-MM-DD).
//
// ok=false means the value could not be interpreted; the caller omits the column
// rather than failing. That matters: a birthday is not load-bearing for login, and
// before this the whole INSERT failed, so one unparseable date cost a member their
// entire row — and therefore their ability to log in at all.
//
// # Why the timezone conversion
//
// The RFC3339 values are midnight *Copenhagen* time expressed in UTC: 22:00Z in
// summer (UTC+2) and 23:00Z in winter (UTC+1), which is exactly what the dead-lettered
// values showed. Naively truncating to the first ten characters would therefore record
// the day *before* the real birthday for every member whose record was stored this
// way. Converting to Copenhagen recovers the intended calendar date.
//
// If the location cannot be loaded (no tzdata in the binary) we fall back to the UTC
// date rather than dropping the value. That is the off-by-one, accepted knowingly as
// the lesser loss — and cmd/api embeds tzdata precisely so it does not happen.
func parseBirthday(raw string) (string, bool) {
	if raw == "" {
		return "", false
	}

	for _, layout := range birthdayFormats {
		t, err := time.Parse(layout, raw)
		if err != nil {
			continue
		}
		// A plain date parses with a zero clock; there is nothing to convert and
		// converting would be wrong (it would shift the date backwards).
		if layout == birthdayFormats[0] {
			return t.Format("2006-01-02"), true
		}
		if loc, lerr := time.LoadLocation(birthdayLocation); lerr == nil {
			return t.In(loc).Format("2006-01-02"), true
		}
		return t.Format("2006-01-02"), true
	}
	return "", false
}

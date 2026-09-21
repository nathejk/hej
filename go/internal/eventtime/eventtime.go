// Package eventtime is the one place that knows what timezone this event's clocks are in.
//
// # Why this exists (task 358)
//
// The maintainer's instruction, 2026-09-21: *"all timestamps shall be printed in CEST tz"*. Before it, every
// timestamp on the public site was rendered in **UTC** — two hours early all through the night the pages are
// about. Measured rather than reasoned: patrol 71's first scan is epoch 1789761524, which is 21:58:44 CEST, and
// the patrol page printed *kl. 19:58*.
//
// The cause is ordinary and worth naming, because it is how this comes back: nothing was *converting* anything.
// Scans are stored as Unix seconds, `time.Unix` returns an instant in the process's local zone, the container has
// no `/etc/timezone`, so local is UTC — and the formatter printed whatever zone its input happened to carry. A
// rendering that inherits its timezone from its input has no timezone at all.
//
// # CEST, or Europe/Copenhagen?
//
// The instruction says CEST. This loads **Europe/Copenhagen**, which renders CEST in summer and CET in winter,
// and that is the instruction rather than a deviation from it: CEST *is* Copenhagen's clock in September, when
// the event happens. Hardcoding +02:00 would be wrong for a page still being read in November — and the page is
// the archive, so it will be.
package eventtime

import (
	"fmt"
	"sync"
	"time"
)

// zone is the IANA name. One constant, so a grep for the zone finds one answer.
const zone = "Europe/Copenhagen"

var (
	once     sync.Once
	location *time.Location
	loadErr  error
)

// Location is the timezone every rendered timestamp is converted to.
//
// Loaded once and cached: `LoadLocation` reads the zoneinfo database from disk, and this is called for every
// timestamp on every public page.
//
// # The fallback is UTC, and it is a deployment fault
//
// A missing zoneinfo database (a scratch image without tzdata) leaves this returning UTC, which is the wrong
// clock by an hour or two — but it is the same wrong clock everywhere, and every alternative is worse: panicking
// takes the site down over a cosmetic fault, and a hardcoded offset would be silently wrong twice a year
// forever. LoadErr exists so a caller can say so out loud at boot rather than leaving it to be found on a page.
func Location() *time.Location {
	once.Do(func() {
		loc, err := time.LoadLocation(zone)
		if err != nil {
			location, loadErr = time.UTC, err
			return
		}
		location = loc
	})
	return location
}

// LoadErr reports why Location fell back to UTC, or nil.
//
// Checked at boot so the failure is a log line somebody can act on. Nothing renders differently because of it.
func LoadErr() error {
	Location()
	return loadErr
}

// In converts an instant to the event's zone.
//
// The whole point of the package in one line: **call this at every rendering boundary**, not where a value is
// read from the database or the stream. Instants stay instants internally; a timezone is a property of how a
// human is shown a moment, not of the moment.
func In(t time.Time) time.Time { return t.In(Location()) }

// danishMonths are the month names, lowercase as Danish writes them.
//
// Not `time.Format`, and this is the second bug this package fixed: `glimtpublic.go` formatted dates with the
// layout `"2. januar 2006 kl. 15:04"`, and **`januar` is not a layout token**. Go copied it through as a
// literal, so every public glimt was dated in January — a September photograph read *"18. januar 2026"*. It
// carried a comment claiming it matched the other surface's format, which is exactly what stopped anybody
// checking.
//
// Go's own month names are English and it has no locale support, so a table is the only honest option. A test
// walks all twelve.
var danishMonths = [12]string{
	"januar", "februar", "marts", "april", "maj", "juni",
	"juli", "august", "september", "oktober", "november", "december",
}

// Danish renders an instant as the public pages say it: "18. september 2026 kl. 21:58".
//
// Converts first, always. A 24-hour clock, because that is what a Dane reads and because an event running past
// midnight makes am/pm a puzzle.
func Danish(t time.Time) string {
	t = In(t)
	return fmt.Sprintf("%d. %s %d kl. %02d:%02d",
		t.Day(), danishMonths[int(t.Month())-1], t.Year(), t.Hour(), t.Minute())
}

// Clock renders just the time of day: "21:58".
//
// For a list whose heading already establishes the date. Converts, like everything here.
func Clock(t time.Time) string {
	t = In(t)
	return fmt.Sprintf("%02d:%02d", t.Hour(), t.Minute())
}

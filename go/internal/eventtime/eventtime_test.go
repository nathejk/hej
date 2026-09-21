package eventtime

import (
	"strings"
	"testing"
	"time"
)

// The measurement this package was written from: patrol 71's first scan of 2026.
//
// Epoch 1789761524. The patrol page printed *kl. 19:58* — UTC — where the scan happened at 21:58 CEST.
const firstScan2026 = 1789761524

func TestTheScanThatShowedTheBugNowRendersInCEST(t *testing.T) {
	got := Danish(time.Unix(firstScan2026, 0).UTC())
	if want := "18. september 2026 kl. 21:58"; got != want {
		t.Errorf("Danish = %q, want %q", got, want)
	}
}

// **The input's own zone must not matter.** This is the actual defect: the old formatter printed whatever zone
// its argument happened to carry, so the same instant read two ways rendered two ways.
func TestTheInputsZoneDoesNotChangeTheOutput(t *testing.T) {
	instant := time.Unix(firstScan2026, 0)

	tokyo, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Skipf("no zoneinfo for Asia/Tokyo: %v", err)
	}

	for _, in := range []time.Time{instant.UTC(), instant.In(tokyo), instant.In(Location())} {
		if got, want := Danish(in), "18. september 2026 kl. 21:58"; got != want {
			t.Errorf("Danish(%s) = %q, want %q", in.Location(), got, want)
		}
		if got, want := Clock(in), "21:58"; got != want {
			t.Errorf("Clock(%s) = %q, want %q", in.Location(), got, want)
		}
	}
}

// Winter is CET, and that is why this loads a zone rather than pinning +02:00. The instruction said CEST because
// the event is in September; a page read in November must still say the right time.
func TestWinterRendersInCET(t *testing.T) {
	if LoadErr() != nil {
		t.Skipf("no zoneinfo: %v", LoadErr())
	}

	// 1 January 2026, 12:00 UTC → 13:00 in Copenhagen (CET, +1).
	winter := time.Date(2026, time.January, 1, 12, 0, 0, 0, time.UTC)
	if got, want := Clock(winter), "13:00"; got != want {
		t.Errorf("Clock in winter = %q, want %q (CET is +1)", got, want)
	}

	// And summer is +2, so the offset is genuinely doing the work.
	summer := time.Date(2026, time.July, 1, 12, 0, 0, 0, time.UTC)
	if got, want := Clock(summer), "14:00"; got != want {
		t.Errorf("Clock in summer = %q, want %q (CEST is +2)", got, want)
	}
}

// The race night crosses midnight, which is the case a wrong conversion is most visible in: a scan at 00:30 CEST
// is 22:30 UTC *the day before*, so getting this wrong moves the date as well as the clock.
func TestAScanAfterMidnightKeepsTheRightDate(t *testing.T) {
	// 2026-09-18 22:30 UTC = 2026-09-19 00:30 CEST.
	instant := time.Date(2026, time.September, 18, 22, 30, 0, 0, time.UTC)
	if got, want := Danish(instant), "19. september 2026 kl. 00:30"; got != want {
		t.Errorf("Danish = %q, want %q", got, want)
	}
}

// All twelve months, in Danish. The bug this replaces printed "januar" for every one of them, because `januar`
// is not a layout token — so a table that is right for January and wrong for the rest would have looked fine in
// exactly the test somebody would have written.
func TestEveryMonthIsSpelledInDanish(t *testing.T) {
	want := []string{
		"januar", "februar", "marts", "april", "maj", "juni",
		"juli", "august", "september", "oktober", "november", "december",
	}
	for i, month := range want {
		// The 15th at noon, so no conversion can move the date into a neighbouring month.
		got := Danish(time.Date(2026, time.Month(i+1), 15, 12, 0, 0, 0, time.UTC))
		if !strings.Contains(got, month) {
			t.Errorf("month %d rendered %q, want it to contain %q", i+1, got, month)
		}
	}
}

// The Go layout trap, pinned so nobody "simplifies" the table back into a format string. If this ever passes,
// Go has grown locale support and the table can go.
func TestGoCannotSpellDanishMonths(t *testing.T) {
	september := time.Date(2026, time.September, 18, 21, 58, 0, 0, time.UTC)
	if got := september.Format("2. januar 2006"); !strings.Contains(got, "januar") {
		t.Skip("time.Format now localises month names; the table in this package can be replaced")
	} else if strings.Contains(got, "september") {
		t.Errorf("unexpected: %q contains the right month", got)
	}
}

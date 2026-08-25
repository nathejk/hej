package person

import "testing"

// The values in the want column are the real dead-lettered inputs from the first
// replay against production data, with the date the member was actually born.
//
// The timezone conversion is the point: 22:00Z is midnight Copenhagen in summer
// (UTC+2) and 23:00Z is midnight in winter (UTC+1), so truncating the string to ten
// characters would record the day *before* every one of these birthdays.
func TestParseBirthday(t *testing.T) {
	tests := []struct {
		name   string
		raw    string
		want   string
		wantOK bool
	}{
		{"plain date passes through", "2010-06-04", "2010-06-04", true},
		{"summer offset rolls to the next day", "2010-06-03T22:00:00.000Z", "2010-06-04", true},
		{"winter offset rolls to the next day", "2011-02-09T23:00:00.000Z", "2011-02-10", true},
		{"august", "2010-08-01T22:00:00.000Z", "2010-08-02", true},
		{"may", "2012-05-02T22:00:00.000Z", "2012-05-03", true},
		{"rfc3339 without millis", "2012-06-07T22:00:00Z", "2012-06-08", true},

		{"empty", "", "", false},
		{"garbage", "not a date", "", false},
		{"partial", "2010-06", "", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parseBirthday(tc.raw)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if got != tc.want {
				t.Fatalf("parseBirthday(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}

// A plain date must NOT be timezone-shifted. It parses with a zero clock, so
// converting it to Copenhagen would move it backwards a day — the opposite of the bug
// this function exists to fix.
func TestParseBirthdayDoesNotShiftPlainDates(t *testing.T) {
	for _, d := range []string{"2010-01-01", "2010-06-15", "2010-12-31"} {
		got, ok := parseBirthday(d)
		if !ok {
			t.Fatalf("%q should parse", d)
		}
		if got != d {
			t.Fatalf("plain date %q was shifted to %q", d, got)
		}
	}
}

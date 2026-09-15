package main

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

// captureLogs returns a logger writing JSON into a buffer, plus the buffer.
//
// JSON rather than text so assertions can be made on the *fields* — the numbers are the point of these log
// lines, and a substring match on a rendered message would pass on a line that carried the wrong count.
func captureLogs() (*slog.Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	return slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})), &buf
}

type logLine struct {
	Level        string  `json:"level"`
	Msg          string  `json:"msg"`
	Year         string  `json:"year"`
	Sets         float64 `json:"sets"`
	PatrolSets   float64 `json:"patrol_sets"`
	Sheets       float64 `json:"sheets"`
	PatrolSheets float64 `json:"patrol_sheets"`
	Unattributed float64 `json:"unattributed"`
	Total        float64 `json:"total"`
}

func lines(t *testing.T, buf *bytes.Buffer) []logLine {
	t.Helper()
	var out []logLine
	for _, raw := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if raw == "" {
			continue
		}
		var l logLine
		if err := json.Unmarshal([]byte(raw), &l); err != nil {
			t.Fatalf("log line is not JSON: %q: %v", raw, err)
		}
		out = append(out, l)
	}
	return out
}

func only(t *testing.T, buf *bytes.Buffer) logLine {
	t.Helper()
	got := lines(t, buf)
	if len(got) != 1 {
		t.Fatalf("want exactly one log line, got %d: %s", len(got), buf.String())
	}
	return got[0]
}

// The disaster this reporting exists for: sheets are defined, but none is in a set marked for patrols — so
// the handout list and the reveal rule both filter everything out and every patrol sees an empty map, while
// every other number looks healthy.
func TestMapCounts_NoPatrolSetIsAWarning(t *testing.T) {
	logger, buf := captureLogs()

	mapCountsReporter(logger)("2026", 2 /*sets*/, 0 /*patrolSets*/, 12 /*sheets*/, 0 /*patrolSheets*/)

	got := only(t, buf)
	if got.Level != "WARN" {
		t.Errorf("level = %q, want WARN: somebody has to go and mark a set", got.Level)
	}
	if got.Sheets != 12 || got.Sets != 2 {
		t.Errorf("counts not reported: %+v", got)
	}
	// The message has to name the value to look for, or the operator is left guessing which of four
	// team types is meant.
	if !strings.Contains(buf.String(), "patrulje") {
		t.Errorf("want the expected team type named: %s", buf.String())
	}
}

// A year with no sheets at all is information, not a warning. Early in the season it is simply the truth, and
// a warning that fires for months trains people to ignore the channel.
func TestMapCounts_NoSheetsYetIsInfo(t *testing.T) {
	logger, buf := captureLogs()

	mapCountsReporter(logger)("2026", 0, 0, 0, 0)

	got := only(t, buf)
	if got.Level != "INFO" {
		t.Errorf("level = %q, want INFO", got.Level)
	}
}

// A marked set that holds no sheets is a half-finished setup: somebody made the set and stopped.
func TestMapCounts_EmptyPatrolSetIsAWarning(t *testing.T) {
	logger, buf := captureLogs()

	mapCountsReporter(logger)("2026", 2, 1, 12, 0)

	got := only(t, buf)
	if got.Level != "WARN" {
		t.Errorf("level = %q, want WARN", got.Level)
	}
}

func TestMapCounts_HealthyYearIsInfoWithTheNumbers(t *testing.T) {
	logger, buf := captureLogs()

	mapCountsReporter(logger)("2026", 2, 1, 12, 7)

	got := only(t, buf)
	if got.Level != "INFO" {
		t.Errorf("level = %q, want INFO", got.Level)
	}
	if got.PatrolSheets != 7 || got.PatrolSets != 1 {
		t.Errorf("want the numbers an operator can check against the print run: %+v", got)
	}
}

// The upstream gap that is invisible from inside this repo: with the personnel rota missing, scans attribute
// to no checkpoint, so post names and on-time verdicts silently disappear and reveal rule 3 never fires.
func TestScanAttribution_ManyUnattributedIsAWarning(t *testing.T) {
	logger, buf := captureLogs()

	reportScanAttribution(logger, nil, "2026") // nil table: nothing to say
	if buf.Len() != 0 {
		t.Fatalf("a missing projection is already logged at construction: %s", buf.String())
	}
}

func TestScanAttribution_RatioDecidesTheLevel(t *testing.T) {
	cases := []struct {
		name                string
		unattributed, total int
		wantLevel           string
	}{
		// A healthy event: a handful of scans by someone whose shift was recorded loosely.
		{"a few", 40, 900, "INFO"},
		// A rota with several posts missing. Well above PRD 016 §9's ≤ 5 % target.
		{"a fifth", 180, 900, "WARN"},
		// The rota was never filled in at all.
		{"all of them", 900, 900, "WARN"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			logger, buf := captureLogs()

			reportAttributionCounts(logger, "2026", c.unattributed, c.total)

			got := only(t, buf)
			if got.Level != c.wantLevel {
				t.Errorf("%d of %d: level = %q, want %q", c.unattributed, c.total, got.Level, c.wantLevel)
			}
			if got.Unattributed != float64(c.unattributed) || got.Total != float64(c.total) {
				t.Errorf("want the ratio's both halves reported: %+v", got)
			}
		})
	}
}

// Before the race there are no scans. Not worth a line: a startup "everything is fine" trains people to
// ignore the channel just as effectively as a periodic one.
func TestScanAttribution_NoScansIsSilent(t *testing.T) {
	logger, buf := captureLogs()

	reportAttributionCounts(logger, "2026", 0, 0)

	if buf.Len() != 0 {
		t.Errorf("want silence before the race, got: %s", buf.String())
	}
}

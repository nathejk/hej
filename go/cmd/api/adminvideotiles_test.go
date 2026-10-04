package main

import (
	"strings"
	"testing"

	"nathejk.dk/nathejk/table/photo"
)

// The contact sheet's video tiles (PRD 029, task 497).
func TestTheContactSheetShowsVideoStates(t *testing.T) {
	_, srv := libraryApp(t, &libraryCurator{rows: []photo.LibraryPhoto{
		{ID: strings.Repeat("a", 64), Kind: "video", Status: "processing", DurationMs: 95_000},
		{ID: strings.Repeat("b", 64), Kind: "video", Status: "failed", FailReason: "moov atom not found", DurationMs: 3000},
		{ID: strings.Repeat("c", 64), Ref: strings.Repeat("c", 64), Kind: "video", Status: "ready", DurationMs: 22*60*1000 + 5000},
	}})
	body := adminBody(t, getAdmin(t, srv, "/admin/fragments/photos", testAdminUser, testAdminPass))

	for _, want := range []string{
		`data-kind="video" data-status="processing"`, "behandles…", "▶ 1:35",
		`data-status="failed"`, `title="moov atom not found"`,
		"▶ 22:05",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the sheet lacks %q", want)
		}
	}
	// A tile with no poster yet must not ask for one: the media route would 404 into a broken image.
	if strings.Contains(body, `src="/api/admin/photos/`+strings.Repeat("a", 64)) {
		t.Error("a processing video's tile requests a thumbnail it does not have")
	}
	if !strings.Contains(body, `src="/api/admin/photos/`+strings.Repeat("c", 64)+`/media?variant=thumb`) {
		t.Error("a ready video's tile should show its poster")
	}
}

func TestFormatDuration(t *testing.T) {
	for ms, want := range map[int]string{0: "0:00", 59_999: "0:59", 95_000: "1:35", 3_725_000: "1:02:05"} {
		if got := formatDuration(ms); got != want {
			t.Errorf("%d: %s, want %s", ms, got, want)
		}
	}
}

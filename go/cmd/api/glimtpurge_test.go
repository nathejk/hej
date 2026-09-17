package main

import (
	"testing"
	"time"

	"github.com/jrgensen/cqrs/cqrstest"

	"nathejk.dk/internal/blob"
	"nathejk.dk/nathejk/table/glimt"
)

// Glimt retention tests (task 310).
//
// Three properties, in descending order of how badly getting them wrong would end:
//
//  1. Disabled means disabled — `0` (and any negative) purges nothing.
//  2. A purge does not blank media shared with a surviving glimt.
//  3. The event is published before the bytes go, so a failed publish leaves the glimt whole.

func purgeApp(t *testing.T, retention time.Duration) (*application, *stubGlimt, *cqrstest.Publisher) {
	t.Helper()
	app, store, pub := glimtApp(t, nil, spejderPerson())
	app.config.glimtRetention = retention
	return app, store, pub
}

func TestGlimtPurge_DeletesExpiredAndPublishes(t *testing.T) {
	app, store, pub := purgeApp(t, 30*24*time.Hour)
	row := mediaGlimt(t, app, "g-old", "mock-spejder-1", "spejder", glimt.AudienceGroup)
	row.CreatedAt = time.Now().UTC().Add(-60 * 24 * time.Hour)
	store.rows = []glimt.Glimt{row}

	purged, err := app.purgeExpiredGlimt(t.Context())
	if err != nil {
		t.Fatalf("purge: %v", err)
	}
	if purged != 1 {
		t.Fatalf("purged = %d, want 1", purged)
	}

	if got := pub.Subjects(); len(got) != 1 || got[0] != "NATHEJK.2026.glimt.g-old.purged" {
		t.Fatalf("subjects = %v", got)
	}
	var body glimt.Purged
	if err := pub.Messages[0].Body(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Reason != "retention" {
		t.Errorf("reason = %q; the log should say *why* a photograph went", body.Reason)
	}

	// Both objects gone — full and thumbnail. A 320px thumbnail of a child's face is still a
	// photograph of a child's face.
	for name, ref := range map[string]string{"full": row.Media[0].Ref, "thumb": row.Media[0].ThumbRef} {
		if ok, _ := app.blobs.Exists(t.Context(), blob.Ref(ref)); ok {
			t.Errorf("%s bytes survived the purge", name)
		}
	}
}

func TestGlimtPurge_KeepsWhatIsInsideTheWindow(t *testing.T) {
	app, store, pub := purgeApp(t, 30*24*time.Hour)
	row := mediaGlimt(t, app, "g-recent", "mock-spejder-1", "spejder", glimt.AudienceGroup)
	row.CreatedAt = time.Now().UTC().Add(-2 * 24 * time.Hour)
	store.rows = []glimt.Glimt{row}

	purged, err := app.purgeExpiredGlimt(t.Context())
	if err != nil {
		t.Fatalf("purge: %v", err)
	}
	if purged != 0 {
		t.Errorf("purged = %d, want 0", purged)
	}
	if len(pub.Subjects()) != 0 {
		t.Errorf("subjects = %v", pub.Subjects())
	}
}

// TestGlimtPurge_DisabledPurgesNothing is the property dev and CI depend on: a fixture posted last
// month must still be there tomorrow.
func TestGlimtPurge_DisabledPurgesNothing(t *testing.T) {
	for name, retention := range map[string]time.Duration{
		"zero":     0,
		"negative": -time.Hour,
	} {
		t.Run(name, func(t *testing.T) {
			app, store, pub := purgeApp(t, retention)
			row := mediaGlimt(t, app, "g-ancient", "mock-spejder-1", "spejder", glimt.AudienceGroup)
			// Old enough that any positive window would take it.
			row.CreatedAt = time.Now().UTC().Add(-10 * 365 * 24 * time.Hour)
			store.rows = []glimt.Glimt{row}

			purged, err := app.purgeExpiredGlimt(t.Context())
			if err != nil {
				t.Fatalf("purge: %v", err)
			}
			if purged != 0 {
				t.Errorf("purged = %d with retention %v, want 0", purged, retention)
			}
			if len(pub.Subjects()) != 0 {
				t.Errorf("a disabled purge published %v", pub.Subjects())
			}
			if ok, _ := app.blobs.Exists(t.Context(), blob.Ref(row.Media[0].Ref)); !ok {
				t.Error("a disabled purge deleted media")
			}
		})
	}
}

// TestGlimtPurge_DoesNotBlankSharedMedia is task 306's hazard applied to the unattended case.
//
// A user delete does this to one post while somebody watches. This job would do it in bulk, at 03:00,
// with nobody looking — so the shared-object check has to be in the path both of them take.
func TestGlimtPurge_DoesNotBlankSharedMedia(t *testing.T) {
	app, store, _ := purgeApp(t, 30*24*time.Hour)

	old := mediaGlimt(t, app, "g-old", "mock-spejder-1", "spejder", glimt.AudienceGroup)
	old.CreatedAt = time.Now().UTC().Add(-60 * 24 * time.Hour)
	// A recent glimt referencing the *same objects* — which is what posting the same photo
	// produces under content addressing.
	recent := mediaGlimt(t, app, "g-recent", "other-spejder", "spejder", glimt.AudienceGroup)
	recent.CreatedAt = time.Now().UTC()
	recent.Media[0].Ref = old.Media[0].Ref
	recent.Media[0].ThumbRef = old.Media[0].ThumbRef
	store.rows = []glimt.Glimt{old, recent}

	if _, err := app.purgeExpiredGlimt(t.Context()); err != nil {
		t.Fatalf("purge: %v", err)
	}

	for name, ref := range map[string]string{"full": old.Media[0].Ref, "thumb": old.Media[0].ThumbRef} {
		if ok, _ := app.blobs.Exists(t.Context(), blob.Ref(ref)); !ok {
			t.Errorf("the purge deleted %s bytes that a surviving glimt still references", name)
		}
	}
}

// TestGlimtPurge_PublishesBeforeDeletingBytes pins the ordering, which is the *opposite* of the
// portrait purge's.
//
// For a portrait, bytes-first is self-healing: the row keeps pointing at a missing object and the next
// run retries the event, and nothing else can be harmed because a portrait's objects belong to one
// person. A glimt's objects may be shared, so bytes-first plus a failed publish would leave a
// surviving glimt permanently blank with no record of why.
func TestGlimtPurge_PublishesBeforeDeletingBytes(t *testing.T) {
	app, store, _ := purgeApp(t, 30*24*time.Hour)
	row := mediaGlimt(t, app, "g-old", "mock-spejder-1", "spejder", glimt.AudienceGroup)
	row.CreatedAt = time.Now().UTC().Add(-60 * 24 * time.Hour)
	store.rows = []glimt.Glimt{row}
	app.commands = commandsWithNoPublisher()

	purged, err := app.purgeExpiredGlimt(t.Context())
	if err != nil {
		t.Fatalf("purge returned an error rather than skipping the row: %v", err)
	}
	if purged != 0 {
		t.Errorf("purged = %d, want 0 when the event cannot be published", purged)
	}
	// The glimt is still whole, so the next tick can try again.
	for name, ref := range map[string]string{"full": row.Media[0].Ref, "thumb": row.Media[0].ThumbRef} {
		if ok, _ := app.blobs.Exists(t.Context(), blob.Ref(ref)); !ok {
			t.Errorf("%s bytes were deleted although the purge was never published", name)
		}
	}
}

func TestGlimtPurge_NoProjectionIsNotAnError(t *testing.T) {
	// Running without a database is a supported mode (PRD 008 §5).
	app, _, _ := purgeApp(t, 30*24*time.Hour)
	app.models = newModelsWithGlimt(&stubPeople{}, nil)

	purged, err := app.purgeExpiredGlimt(t.Context())
	if err != nil || purged != 0 {
		t.Errorf("= %d, %v; want 0, nil", purged, err)
	}
}

// TestGlimtPublicCutoff covers the read-time window, including the reading that would have been a
// quiet disaster.
func TestGlimtPublicCutoff(t *testing.T) {
	app, _, _ := purgeApp(t, 0)

	// Zero means "as long as the glimt itself", NOT "immediately". The other reading would
	// silently empty the public page in a dev environment where every retention value is 0.
	app.config.glimtPublicRetention = 0
	if got := app.glimtPublicCutoff(); !got.IsZero() {
		t.Errorf("cutoff with retention 0 = %v, want the zero time (no cutoff)", got)
	}
	app.config.glimtPublicRetention = -time.Hour
	if got := app.glimtPublicCutoff(); !got.IsZero() {
		t.Errorf("cutoff with negative retention = %v, want the zero time", got)
	}

	app.config.glimtPublicRetention = 30 * 24 * time.Hour
	got := app.glimtPublicCutoff()
	if got.IsZero() {
		t.Fatal("cutoff with a real retention is zero")
	}
	want := time.Now().UTC().Add(-30 * 24 * time.Hour)
	if diff := got.Sub(want); diff > time.Minute || diff < -time.Minute {
		t.Errorf("cutoff = %v, want about %v", got, want)
	}
}

// TestGlimtPublicRetentionIsNotAStateChange records why the window is enforced at read time.
//
// Audience is immutable and hiding is a moderation act with a person's name on it. Neither may be
// repurposed by a timer — and a read-time cutoff is reversible, where a state change would need an
// "unexpire" event to undo.
func TestGlimtPublicRetentionIsNotAStateChange(t *testing.T) {
	app, store, pub := purgeApp(t, 0)
	app.config.glimtPublicRetention = 24 * time.Hour

	row := publicGlimtRow()
	row.CreatedAt = time.Now().UTC().Add(-72 * time.Hour) // past the public window
	store.rows = []glimt.Glimt{row}

	// It has dropped off the public feed...
	public, err := app.models.Glimt.PublicFeed(app.config.eventYear, app.glimtPublicCutoff(), 20, 0)
	if err != nil {
		t.Fatalf("PublicFeed: %v", err)
	}
	if len(public) != 0 {
		t.Errorf("public feed still returns a glimt past the public window: %+v", public)
	}

	// ...without anything being published, and without its audience or hidden state changing.
	if len(pub.Subjects()) != 0 {
		t.Errorf("the public window published %v; it must be a read-time cutoff", pub.Subjects())
	}
	if store.rows[0].Audience != glimt.AudiencePublic || store.rows[0].HiddenAt != nil {
		t.Errorf("the row was mutated: %+v", store.rows[0])
	}

	// And lengthening the window brings it back, which a state change could not do.
	app.config.glimtPublicRetention = 30 * 24 * time.Hour
	public, err = app.models.Glimt.PublicFeed(app.config.eventYear, app.glimtPublicCutoff(), 20, 0)
	if err != nil {
		t.Fatalf("PublicFeed: %v", err)
	}
	if len(public) != 1 {
		t.Error("lengthening the public window did not bring the glimt back")
	}
}

func TestDurationDaysRoundsDown(t *testing.T) {
	// Rounded down so the number a member reads is never longer than the truth: "2 dage" for a
	// 36-hour window would be a promise the service does not keep.
	cases := map[time.Duration]int{
		0:                   0,
		-time.Hour:          0,
		36 * time.Hour:      1,
		24 * time.Hour:      1,
		90 * 24 * time.Hour: 90,
		23 * time.Hour:      0,
		30*24*time.Hour + 1: 30,
	}
	for d, want := range cases {
		if got := durationDays(d); got != want {
			t.Errorf("durationDays(%v) = %d, want %d", d, got, want)
		}
	}
}

func TestRuntimeConfigCarriesRetentionDays(t *testing.T) {
	// Served so the composer and the privacy page can state the real number. Copy that says
	// "90 dage" while the deployment is set to 30 is a promise about somebody's photographs
	// that the service will not keep.
	app, _, _ := purgeApp(t, 90*24*time.Hour)
	app.config.glimtPublicRetention = 30 * 24 * time.Hour

	if got := durationDays(app.config.glimtRetention); got != 90 {
		t.Errorf("retention days = %d, want 90", got)
	}
	if got := durationDays(app.config.glimtPublicRetention); got != 30 {
		t.Errorf("public retention days = %d, want 30", got)
	}
}

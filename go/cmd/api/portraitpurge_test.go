package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jrgensen/cqrs/cqrstest"

	"nathejk.dk/internal/blob"
	"nathejk.dk/nathejk/table/person"
)

// purgeTestApp is an app with a fixed retention window and a stub projection.
func purgeTestApp(t *testing.T, people *stubPeople, pub *cqrstest.Publisher) *application {
	t.Helper()
	app := photoTestApp(t, pub, people)
	app.config.portraitRetention = 30 * 24 * time.Hour
	return app
}

func TestPurgeDeletesBytesAndPublishesTheEvent(t *testing.T) {
	pub := &cqrstest.Publisher{}
	people := &stubPeople{}
	app := purgeTestApp(t, people, pub)

	full := []byte("an expired portrait")
	thumb := []byte("its thumbnail")
	fullRef, err := app.blobs.Put(context.Background(), full)
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	thumbRef, err := app.blobs.Put(context.Background(), thumb)
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	people.expired = []person.ExpiredPortrait{
		{PersonID: "member-1", Ref: fullRef.String(), ThumbRef: thumbRef.String()},
	}

	purged, err := app.purgeExpiredPortraits(context.Background())
	if err != nil {
		t.Fatalf("purge: %v", err)
	}
	if purged != 1 {
		t.Fatalf("purged = %d, want 1", purged)
	}

	// Both objects gone. Forgetting the thumbnail would leave a recognisable face on
	// disk while technically having "deleted the portrait".
	for name, ref := range map[string]blob.Ref{"full": fullRef, "thumb": thumbRef} {
		if ok, _ := app.blobs.Exists(context.Background(), ref); ok {
			t.Errorf("%s bytes still present after purge", name)
		}
	}

	if got := pub.Subjects(); len(got) != 1 || got[0] != "NATHEJK.2026.portrait.member-1.purged" {
		t.Fatalf("subjects = %v", got)
	}
	var body person.PortraitPurged
	if err := pub.Messages[0].Body(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.PersonID != "member-1" || body.Ref != fullRef.String() {
		t.Errorf("event = %+v", body)
	}
	if body.Reason == "" || body.PurgedAt.IsZero() {
		t.Errorf("a deletion of personal data must record why and when: %+v", body)
	}
}

// The cutoff is the policy, so it is worth asserting rather than trusting.
func TestPurgeAsksForTheConfiguredWindow(t *testing.T) {
	people := &stubPeople{}
	app := purgeTestApp(t, people, &cqrstest.Publisher{})
	app.config.portraitRetention = 48 * time.Hour

	before := time.Now().UTC()
	if _, err := app.purgeExpiredPortraits(context.Background()); err != nil {
		t.Fatalf("purge: %v", err)
	}
	if len(people.expiredCutoffs) != 1 {
		t.Fatalf("expected one query, got %d", len(people.expiredCutoffs))
	}

	cutoff := people.expiredCutoffs[0]
	want := before.Add(-48 * time.Hour)
	if cutoff.Sub(want) > time.Minute || want.Sub(cutoff) > time.Minute {
		t.Errorf("cutoff = %v, want about %v", cutoff, want)
	}
}

// Re-running must be safe: the blob store treats an absent object as deleted, so an
// interrupted run leaves work the next one simply finishes.
func TestPurgeIsIdempotent(t *testing.T) {
	people := &stubPeople{}
	app := purgeTestApp(t, people, &cqrstest.Publisher{})

	ref, err := app.blobs.Put(context.Background(), []byte("bytes"))
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	expired := []person.ExpiredPortrait{{PersonID: "member-1", Ref: ref.String()}}

	people.expired = expired
	if _, err := app.purgeExpiredPortraits(context.Background()); err != nil {
		t.Fatalf("first purge: %v", err)
	}
	// The same row again, as would happen if the purge event had not been projected yet.
	people.expired = expired
	if _, err := app.purgeExpiredPortraits(context.Background()); err != nil {
		t.Fatalf("second purge: %v", err)
	}
}

// A portrait whose row has no thumbnail (captured before task 104) must purge cleanly.
func TestPurgeHandlesAMissingThumbnail(t *testing.T) {
	people := &stubPeople{}
	app := purgeTestApp(t, people, &cqrstest.Publisher{})

	ref, err := app.blobs.Put(context.Background(), []byte("bytes"))
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	people.expired = []person.ExpiredPortrait{{PersonID: "member-1", Ref: ref.String()}}

	if purged, err := app.purgeExpiredPortraits(context.Background()); err != nil || purged != 1 {
		t.Fatalf("purged = %d, err = %v", purged, err)
	}
}

// One undeletable portrait must not stop everybody else's from expiring — and because the
// query re-evaluates, the failure recurs rather than being lost.
func TestPurgeContinuesPastAFailingRow(t *testing.T) {
	people := &stubPeople{}
	// No publisher at all, so publishing the first row fails. The second must still be
	// attempted.
	app := purgeTestApp(t, people, nil)
	app.commands = commandsWithNoPublisher()

	people.expired = []person.ExpiredPortrait{
		{PersonID: "member-1", Ref: "aaaa"},
		{PersonID: "member-2", Ref: "bbbb"},
	}
	purged, err := app.purgeExpiredPortraits(context.Background())
	if err != nil {
		t.Fatalf("the pass itself must not fail: %v", err)
	}
	if purged != 0 {
		t.Errorf("purged = %d, want 0 — nothing could be recorded", purged)
	}
	if len(people.expiredCutoffs) != 1 {
		t.Errorf("expected exactly one query per pass, got %d", len(people.expiredCutoffs))
	}
}

// A ref that is not a content hash must never reach blob.Delete: it is the one value on
// this path that travels from a database row into a filesystem operation.
func TestPurgeSkipsNonHashRefs(t *testing.T) {
	people := &stubPeople{}
	app := purgeTestApp(t, people, &cqrstest.Publisher{})

	people.expired = []person.ExpiredPortrait{
		{PersonID: "member-1", Ref: "../../../etc/passwd", ThumbRef: "also-not-a-hash"},
	}
	// Still counts as purged: the row is cleared by the event, which is the only way to
	// stop it being found again. Leaving it would mean retrying a bad ref forever.
	if purged, err := app.purgeExpiredPortraits(context.Background()); err != nil || purged != 1 {
		t.Fatalf("purged = %d, err = %v", purged, err)
	}
}

// A read failure fails the pass rather than being mistaken for "nothing expired".
func TestPurgeSurfacesAQueryFailure(t *testing.T) {
	people := &stubPeople{expiredErr: errors.New("database gone")}
	app := purgeTestApp(t, people, &cqrstest.Publisher{})

	if _, err := app.purgeExpiredPortraits(context.Background()); err == nil {
		t.Fatal("want an error")
	}
}

// Without a database there is nothing to purge, and that is not an error: running without
// one is a supported mode (PRD 008 §5).
func TestPurgeWithoutAProjectionIsANoop(t *testing.T) {
	app := purgeTestApp(t, nil, &cqrstest.Publisher{})
	app.models.People = nil

	if purged, err := app.purgeExpiredPortraits(context.Background()); err != nil || purged != 0 {
		t.Fatalf("purged = %d, err = %v", purged, err)
	}
}

// Retention of zero disables the loop rather than purging everything immediately — the
// dangerous misreading of "0 = no retention".
func TestRunPortraitPurgeDoesNothingWhenDisabled(t *testing.T) {
	people := &stubPeople{expired: []person.ExpiredPortrait{{PersonID: "member-1", Ref: "aaaa"}}}
	app := purgeTestApp(t, people, &cqrstest.Publisher{})
	app.config.portraitRetention = 0

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	app.runPortraitPurge(ctx, time.Millisecond, quietLogger())

	time.Sleep(20 * time.Millisecond)
	if len(people.expiredCutoffs) != 0 {
		t.Errorf("purge ran despite being disabled: %v", people.expiredCutoffs)
	}
}

package main

import (
	"context"
	"fmt"
	"time"

	"nathejk.dk/internal/blob"
	"nathejk.dk/nathejk/table/person"
)

// storePortrait is the portrait write path (PRD 003, task 103): bytes to the blob
// store, then a domain event carrying the reference.
//
// # Order is not arbitrary
//
// Bytes first, event second. The two failure modes are not symmetrical:
//
//   - Event published, bytes missing → the projection claims a portrait that cannot be
//     served, forever. Every read degrades to "no photo" while the row insists there is
//     one, and no replay can fix it because the log holds only a reference.
//   - Bytes stored, event not published → an unreferenced object. Harmless: nothing
//     points at it, and the retention sweep (task 109) can collect it.
//
// So the recoverable failure is the one we risk.
//
// # No SQL here
//
// The row is written by the person projection consuming the event, never by this
// function. That is the rule from PRD 008 §8, and it is what makes a projection rebuild
// converge: replaying the log re-publishes the same content hash, `Put` of identical
// bytes is a no-op, and the row lands on the same value.
func (app *application) storePortrait(
	ctx context.Context,
	personID string,
	data []byte,
	meta portraitMeta,
) (blob.Ref, error) {
	if personID == "" {
		return "", fmt.Errorf("store portrait: no person")
	}
	if len(data) == 0 {
		return "", fmt.Errorf("store portrait: no data")
	}

	ref, err := app.blobs.Put(ctx, data)
	if err != nil {
		return "", fmt.Errorf("store portrait bytes: %w", err)
	}

	subject, err := person.PortraitSubject(app.config.eventYear, personID)
	if err != nil {
		// A configured year or person id that cannot be a subject token is our
		// problem, not the caller's — and publishing it anyway would make this
		// person's portrait unerasable (see PortraitSubject).
		return "", fmt.Errorf("store portrait: %w", err)
	}

	body := person.PortraitCaptured{
		PersonID:    personID,
		Year:        app.config.eventYear,
		Ref:         ref.String(),
		ContentType: meta.ContentType,
		Bytes:       len(data),
		Width:       meta.Width,
		Height:      meta.Height,
		CapturedAt:  time.Now().UTC(),
	}
	if err := app.commands.Publish(subject, body); err != nil {
		// Deliberately surfaced rather than swallowed: the bytes are stored but nothing
		// references them, so as far as the app is concerned the portrait was NOT
		// saved. Telling the user it was would mean they stop being nudged for a photo
		// that no one can look up — and this is a safety feature.
		return "", fmt.Errorf("publish portrait: %w", err)
	}

	return ref, nil
}

// portraitMeta is what the event records about the stored object beyond its hash.
//
// Dimensions are carried so a consumer — PRD 007's offline thumbnail sync, an audit —
// can reason about the image without fetching it.
type portraitMeta struct {
	ContentType string
	Width       int
	Height      int
}

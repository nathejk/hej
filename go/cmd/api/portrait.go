package main

import (
	"context"
	"fmt"
	"time"

	"nathejk.dk/internal/blob"
	"nathejk.dk/internal/imaging"
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

	// Renditions are stored before the event too, for the same reason as the full image:
	// an event referencing bytes that are not there yet is the unrecoverable order.
	thumbs := make([]person.PortraitThumb, 0, len(meta.Thumbs))
	for _, t := range meta.Thumbs {
		if len(t.Bytes) == 0 {
			continue
		}
		thumbRef, terr := app.blobs.Put(ctx, t.Bytes)
		if terr != nil {
			// Fails the upload rather than storing a portrait with a missing rendition
			// (task 104's requirement). PRD 007 relies on the thumbnails existing for
			// every portrait; letting one through without them would make "incomplete
			// rendition set" a state that has to be handled forever.
			return "", fmt.Errorf("store portrait rendition %q: %w", t.Name, terr)
		}
		thumbs = append(thumbs, person.PortraitThumb{
			Name:        t.Name,
			Ref:         thumbRef.String(),
			ContentType: meta.ContentType,
			Bytes:       len(t.Bytes),
			Width:       t.Width,
			Height:      t.Height,
		})
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
		Thumbs:      thumbs,
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

// hasPortrait reports whether a portrait is on file for this person.
//
// Answers **false** when there is no database rather than failing the profile request:
// the details are still worth showing during an outage, and the worst consequence of a
// false negative here is that the page invites a photo the member has already taken —
// annoying, and much better than a blank profile page.
func (app *application) hasPortrait(personID string) bool {
	if app.models.People == nil {
		return false
	}
	p, found, err := app.models.People.Get(app.config.eventYear, personID)
	if err != nil {
		app.Logger.Error("reading portrait state", "err", err, "userId", personID)
		return false
	}
	return found && p.PortraitRef != ""
}

// peopleOrNil adapts the person projection to the read interface, preserving nil-ness.
//
// Same Go trap as raceAreasOrNil above: a nil *person.Table assigned to an interface is
// not a nil interface, so the handler's `app.models.People == nil` check would pass and
// then dereference.
func peopleOrNil(t *person.Table) person.Queries {
	if t == nil {
		return nil
	}
	return t
}

// portraitMeta is what the event records about the stored objects beyond their hashes,
// plus the rendition bytes to store alongside the full image.
//
// Dimensions are carried so a consumer — PRD 007's offline thumbnail sync, an audit — can
// reason about an image without fetching it.
type portraitMeta struct {
	ContentType string
	Width       int
	Height      int

	// Thumbs are the already-encoded renditions (task 104). Passed in rather than
	// generated here because they come from the *same decode* as the full image, which is
	// what guarantees no rendition disagrees with another about orientation.
	Thumbs []imaging.Rendition
}

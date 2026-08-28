package main

import (
	"context"
	"log/slog"
	"time"

	"nathejk.dk/internal/blob"
	"nathejk.dk/nathejk/table/person"
)

// Portrait retention (PRD 003 §6, task 109).
//
// The rule, decided by the maintainer on 2026-08-28 (task 102): **the portrait does not
// outlive the event.** It is held on a safety basis — identifying a member in the dark,
// including when something has gone wrong — and that purpose expires with the race.
//
// The *number* is configuration rather than a literal here, because it is the part that
// wants a human's answer and the one most likely to change; see `portraitRetention` in
// env.go. What is not configurable is that a purge happens at all.

// purgeBatchSize bounds one pass.
//
// Bounded rather than "delete everything found": a purge run touches the blob store and
// publishes an event per portrait, and after an event there could be ~800 of them at once.
// Batching keeps one pass short and predictable, and the next tick continues — which is
// safe precisely because the query re-evaluates what is still expired.
const purgeBatchSize = 200

// runPortraitPurge deletes expired portraits, then repeats every interval until ctx ends.
//
// # Order: bytes first, event second
//
// The opposite of `storePortrait`, and for the same reason applied to the opposite
// operation. Deleting the bytes and then failing to publish leaves a row pointing at an
// object that is gone: reads degrade to "no photo" (PRD 008 §8), and the **next run finds
// the same row still expired and retries the event** — self-healing. Publishing first and
// then failing to delete would clear the row while the image stayed on disk: an orphaned
// photograph of a minor that nothing points at, so nothing will ever come back for.
//
// For a deletion whose whole purpose is that the bytes are gone, that asymmetry decides
// the order.
func (app *application) runPortraitPurge(ctx context.Context, interval time.Duration, logger *slog.Logger) {
	if app.config.portraitRetention <= 0 {
		logger.Info("portrait retention disabled", "reason", "retention is zero or negative")
		return
	}

	go func() {
		// A short delay before the first pass. Projections replay on boot, and although
		// nothing truncates the person table, starting a deletion loop in the same second
		// as a rebuild is needless: nothing expires in the next minute that was not
		// already expired.
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Minute):
		}

		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			if purged, err := app.purgeExpiredPortraits(ctx); err != nil {
				logger.Error("portrait purge failed", "err", err)
			} else if purged > 0 {
				// Only when something happened. A periodic "purged 0" trains people to
				// ignore the line, and this one should be noticed: it records the
				// deletion of personal data.
				logger.Info("portraits purged",
					"count", purged, "retention", app.config.portraitRetention)
			}

			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

// purgeExpiredPortraits removes one batch of expired portraits and returns how many.
//
// Idempotent and safe to re-run: everything it does is keyed off "is this portrait still
// expired?", and `blob.Delete` treats an absent object as success, so a run interrupted
// halfway leaves work the next run simply finishes.
func (app *application) purgeExpiredPortraits(ctx context.Context) (int, error) {
	if app.models.People == nil {
		// No database, so nothing to read and nothing to purge. Not an error: running
		// without one is a supported mode (PRD 008 §5).
		return 0, nil
	}

	cutoff := time.Now().UTC().Add(-app.config.portraitRetention)
	expired, err := app.models.People.ExpiredPortraits(app.config.eventYear, cutoff, purgeBatchSize)
	if err != nil {
		return 0, err
	}

	purged := 0
	for _, e := range expired {
		if err := app.purgeOnePortrait(ctx, e); err != nil {
			// Keep going. One member's undeletable portrait must not stop everybody
			// else's from expiring — and the failure recurs on the next tick, so it
			// cannot be lost silently.
			app.Logger.Error("purging portrait", "err", err, "personId", e.PersonID)
			continue
		}
		purged++
	}
	return purged, nil
}

func (app *application) purgeOnePortrait(ctx context.Context, e person.ExpiredPortrait) error {
	// Every rendition goes, not just the full image. Forgetting one would leave a
	// recognisable face on disk and technically satisfy "the portrait was deleted", which
	// is the kind of compliance nobody wants to explain. The list comes from
	// Person.PortraitRefs, so adding a thumbnail size cannot bypass this.
	for _, ref := range e.Refs {
		if ref == "" {
			continue
		}
		r := blob.Ref(ref)
		if !r.Valid() {
			// Not a hash, so not something this store put there. Skipped rather than
			// passed to Delete, which is the one place a bad ref could become a path.
			app.Logger.Error("skipping non-hash portrait ref during purge",
				"personId", e.PersonID, "ref", ref)
			continue
		}
		if err := app.blobs.Delete(ctx, r); err != nil {
			return err
		}
	}

	subject, err := person.PortraitPurgeSubject(app.config.eventYear, e.PersonID)
	if err != nil {
		return err
	}
	return app.commands.Publish(subject, person.PortraitPurged{
		PersonID: e.PersonID,
		Year:     app.config.eventYear,
		Refs:     e.Refs,
		Reason:   "retention",
		PurgedAt: time.Now().UTC(),
	})
}

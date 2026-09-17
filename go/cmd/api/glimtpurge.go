package main

import (
	"context"
	"log/slog"
	"time"

	"nathejk.dk/nathejk/table/glimt"
)

// Glimt retention (PRD 019 §6, task 310).
//
// Mirrors `portraitpurge.go`, which already solved this shape. Read that file first: the batching,
// the tick loop and the "log only when something happened" rule all come from it and are not
// re-argued here. What follows is only what differs.
//
// # Two windows, two mechanisms
//
//   - `glimtRetention` is enforced *here*, by deleting media and publishing a purge event.
//   - `glimtPublicRetention` is enforced at **read time**, in the public feed query. Nothing in this
//     file touches it. The reason is in env.go: audience is immutable and hiding is a moderation act
//     with a person's name on it, so neither may be repurposed by a timer — and a read-time cutoff is
//     reversible, where a state change would need an "unexpire" event to undo.
//
// # The hazard this job must not repeat
//
// The blob store is content-addressed, so identical bytes are one object shared by every glimt that
// holds them. Deleting an object because *one* glimt expired would blank the media of any surviving
// glimt referencing it (task 306 found this). A user-initiated delete does that to one post while
// somebody is watching; this job would do it in bulk, at 03:00, unattended. So it reuses the same
// `purgeGlimtBlobs`, which checks `RefsUsedElsewhere` before every deletion.

// glimtPurgeBatchSize bounds one pass, for the reason portraitpurge.go gives: a pass touches the blob
// store and publishes an event per glimt, and after an event there could be thousands rather than the
// portraits' ~800 — a glimt carries up to ten objects, so the byte volume per row is far higher.
const glimtPurgeBatchSize = 100

// runGlimtPurge deletes expired glimt, then repeats every interval until ctx ends.
func (app *application) runGlimtPurge(ctx context.Context, interval time.Duration, logger *slog.Logger) {
	if app.config.glimtRetention <= 0 {
		// Logged, not silent. This is the one configuration value whose failure mode is
		// keeping children's photographs forever, and "off for dev" is indistinguishable from
		// "off because somebody fat-fingered the env" unless the service says which it thinks
		// it is doing.
		logger.Info("glimt retention disabled",
			"reason", "retention is zero or negative",
			"consequence", "glimt and their media are kept indefinitely")
		return
	}

	logger.Info("glimt retention active",
		"retention", app.config.glimtRetention,
		"publicRetention", app.config.glimtPublicRetention)

	go func() {
		// A short delay before the first pass, as the portrait purge does: projections replay
		// on boot, and starting a deletion loop in the same second as a rebuild is needless.
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Minute):
		}

		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			if purged, err := app.purgeExpiredGlimt(ctx); err != nil {
				logger.Error("glimt purge failed", "err", err)
			} else if purged > 0 {
				// Only when something happened, so the line stays worth noticing: it
				// records the deletion of participants' photographs.
				logger.Info("glimt purged", "count", purged, "retention", app.config.glimtRetention)
			}

			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

// purgeExpiredGlimt removes one batch of expired glimt and returns how many.
//
// Idempotent and safe to re-run: everything is keyed off "is this glimt still expired?", `blob.Delete`
// treats an absent object as success, and a run interrupted halfway leaves work the next run finishes.
func (app *application) purgeExpiredGlimt(ctx context.Context) (int, error) {
	if app.models.Glimt == nil {
		// No projection, so nothing to read and nothing to purge. Not an error: running without
		// a database is a supported mode (PRD 008 §5).
		return 0, nil
	}
	if app.config.glimtRetention <= 0 {
		// Belt and braces. The loop above already returned, but this function is also called
		// directly by tests and could be called by a future admin endpoint, and "purge
		// everything" is not a thing a disabled retention should ever be able to mean.
		return 0, nil
	}

	cutoff := time.Now().UTC().Add(-app.config.glimtRetention)
	expired, err := app.models.Glimt.Expired(app.config.eventYear, cutoff, glimtPurgeBatchSize)
	if err != nil {
		return 0, err
	}

	purged := 0
	for _, e := range expired {
		if err := app.purgeOneGlimt(ctx, e); err != nil {
			// Keep going. One undeletable glimt must not stop everybody else's from
			// expiring — and the failure recurs on the next tick, so it cannot be lost
			// silently.
			app.Logger.Error("purging glimt", "err", err, "glimtId", e.GlimtID)
			continue
		}
		purged++
	}
	return purged, nil
}

// purgeOneGlimt publishes the purge and deletes the objects nothing else references.
//
// # Order: event first, bytes second
//
// This is the *opposite* of the portrait purge, whose comment argues for bytes-first — and the
// difference is the shared-object hazard, not a change of mind.
//
// For a portrait, bytes-first is self-healing: the row keeps pointing at a missing object, reads
// degrade to "no photo", and the next run retries the event. Nothing else can be harmed, because a
// portrait's objects belong to exactly one person.
//
// A glimt's objects may be shared with another glimt. If the bytes went first and the publish then
// failed, the retry would come back to a row whose refs have already been deleted — and any surviving
// glimt sharing them is now blank, permanently, with no record of why. Publishing first means a
// failure leaves the glimt intact and the next tick tries again; the worst case is that the purge is
// late, which is exactly the failure a retention job should prefer.
func (app *application) purgeOneGlimt(ctx context.Context, e glimt.Expired) error {
	subject, err := glimt.Subject(app.config.eventYear, e.GlimtID, glimt.VerbPurged)
	if err != nil {
		return err
	}
	if err := app.commands.Publish(subject, glimt.Purged{
		GlimtID:  e.GlimtID,
		Year:     app.config.eventYear,
		Refs:     e.Refs,
		Reason:   "retention",
		PurgedAt: time.Now().UTC(),
	}); err != nil {
		return err
	}

	// Shares the delete path with the user-initiated delete, so the shared-object check cannot
	// be present in one and missing in the other.
	app.purgeGlimtBlobs(ctx, e.GlimtID, e.Refs)
	return nil
}

// glimtPublicCutoff is the timestamp the public feed must not return glimt older than.
//
// Zero time means no cutoff, which is what `glimtPublicRetention <= 0` means: "as long as the glimt
// itself". Deliberately not "immediately" — that reading would silently empty the public page in a
// dev environment where every retention value is set to 0.
func (app *application) glimtPublicCutoff() time.Time {
	if app.config.glimtPublicRetention <= 0 {
		return time.Time{}
	}
	return time.Now().UTC().Add(-app.config.glimtPublicRetention)
}

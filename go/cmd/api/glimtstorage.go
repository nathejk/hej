package main

import (
	"net/http"
)

// The Glimt storage ceilings (PRD 019 §8, §11 Q8, task 311).
//
// # Why a ceiling exists at all
//
// Everything else in this service is a projection replayed from the stream on every boot. The blob
// store is not: it is the one thing that cannot be rebuilt, it lives on one Docker volume, and it is
// therefore the one thing that must be backed up (PRD 008 §8). A feature that lets a thousand
// participants upload video needs a floor under it that is not "the disk filled up at 02:00 during
// the race".
//
// # Rejecting, never evicting
//
// PRD 019 §11 Q8 asks whether a full store should reject the upload or evict the oldest. **Reject.**
// Refusing a photograph in a field is a bad experience; silently deleting somebody else's memories to
// make room is worse, and it would mean this feature's one irreversible operation firing with no
// human involved and nobody told. Retention (task 310) is what frees space, on a schedule everybody
// was told about in advance.
//
// # Two ceilings, two different status codes
//
// The distinction is not pedantry — it decides what the client tells the member to do:
//
//   - **Per member: 429.** It is their quota, and they can act on it: delete something, or wait for
//     retention. The message says so.
//   - **Total: 507.** It is the *server's* problem, and there is nothing the member can do. Telling
//     them they have uploaded too much would be a lie, and telling them to try again invites a loop.
//
// # Fails open, deliberately
//
// If the ceiling cannot be measured — no projection, or a failing query — the upload is **allowed**.
// That is the opposite of how the moderation check fails, and the asymmetry is the point: the cost of
// wrongly allowing an upload is some disk, which retention reclaims and an operator can see, while
// the cost of wrongly refusing one is a member standing in a forest losing a photograph they cannot
// retake. A ceiling is a safety margin, not an authorization.

// glimtCeilingVerdict is why an upload was refused, or that it was not.
type glimtCeilingVerdict int

const (
	glimtCeilingOK glimtCeilingVerdict = iota
	glimtCeilingMember
	glimtCeilingTotal
)

// checkGlimtStorageCeiling reports whether storing `incoming` more bytes for this member is allowed.
//
// `incoming` is the size of what is about to be stored, so the check is against the *result* rather
// than the current state — otherwise a member one byte under their ceiling could upload 12 MiB.
func (app *application) checkGlimtStorageCeiling(personID string, incoming int64) glimtCeilingVerdict {
	memberLimit := app.config.glimtMemberStorageBytes
	totalLimit := app.config.glimtTotalStorageBytes
	if memberLimit <= 0 && totalLimit <= 0 {
		return glimtCeilingOK
	}
	if app.models.Glimt == nil {
		// No projection to measure against. Allowed: see "fails open" above.
		return glimtCeilingOK
	}

	if memberLimit > 0 {
		used, err := app.models.Glimt.StoredBytes(app.config.eventYear, personID)
		if err != nil {
			app.Logger.Warn("could not measure a member's glimt storage; allowing the upload",
				"err", err)
			return glimtCeilingOK
		}
		if used+incoming > memberLimit {
			return glimtCeilingMember
		}
	}

	if totalLimit > 0 {
		used, err := app.models.Glimt.TotalBytes(app.config.eventYear)
		if err != nil {
			app.Logger.Warn("could not measure total glimt storage; allowing the upload", "err", err)
			return glimtCeilingOK
		}
		if used+incoming > totalLimit {
			// Worth a louder line than the member case: this one means the event is out of room,
			// which is an operator problem happening right now.
			app.Logger.Error("the glimt storage ceiling for the event has been reached",
				"used", used, "limit", totalLimit)
			return glimtCeilingTotal
		}
	}

	return glimtCeilingOK
}

// writeGlimtCeilingResponse answers a refused upload, or reports that there was nothing to answer.
//
// Returns true when it has written a response, so the caller can `return` immediately — the same
// shape as `requireGlimtModerator`.
func (app *application) writeGlimtCeilingResponse(
	w http.ResponseWriter, r *http.Request, verdict glimtCeilingVerdict,
) bool {
	switch verdict {
	case glimtCeilingMember:
		// Names the limit and what to do about it. "Du har uploadet for meget" invites a retry that
		// fails identically; saying a member can delete something gives them a way through.
		app.RateLimitMessageResponse(w, r,
			"Du har brugt din plads til billeder. Slet et glimt for at gøre plads.")
		return true
	case glimtCeilingTotal:
		// 507, not 429: there is nothing this member did wrong and nothing they can do.
		app.InsufficientStorageResponse(w, r,
			"Der er ikke mere plads til billeder lige nu. Prøv igen senere.")
		return true
	default:
		return false
	}
}

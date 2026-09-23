package main

import (
	"net/http"

	"nathejk.dk/internal/blob"
)

// The disk floor under an organizer upload (PRD 022 §8.11, §11 Q5; task 384).
//
// # §11 Q5, answered: a floor under the volume, not a quota over the feature
//
// PRD 022 §11 Q5 asks whether a storage ceiling applies to an organizer's upload at all. It does — but not
// the kind glimt has, and the reason is §11 Q2.
//
// `glimtstorage.go`'s ceiling can be a **byte quota** because retention frees space on a schedule everybody
// was told about in advance (PRD 019): a member who hits their allowance has a way through, and the server's
// total allowance is eventually reclaimed. Neither is true here. §11 Q2 resolved that our photographers'
// photographs are **never purged** — nobody was told they would disappear, they are the event's raw record —
// so the store grows monotonically, one event's worth per year, forever. A quota over that would refuse a
// legitimate hand-in the first year the number was too small, with no way through except an operator
// noticing and raising it. The refusal would arrive mid-batch, in front of a photographer, which is the
// exact failure this is supposed to prevent.
//
// What genuinely needs preventing is named in §8.11: *a photographer invited to upload a full card, the disk
// filling mid-batch, and half a hand-in in an inconsistent state with the photographer's card already
// wiped.* That is a property of **the volume**, not of the feature — and asking the filesystem is the only
// measurement that counts the volume's other tenants: portraits, glimt media, and on the production bind
// mount whatever else shares the disk. A per-feature quota is blind to all of them, so it can pass while the
// disk is full.
//
// So: an upload is refused when storing it would leave the volume with less than `ADMIN_DISK_FLOOR_BYTES`
// free. The floor is reserve, not budget. Its job is to make the disk run out **before** anything else on
// this machine does, and to make that happen as a 507 on one photograph rather than as a partial write.
//
// # Fails open, for the same reason the glimt ceiling does
//
// If free space cannot be measured — a store with no volume, a failing `statfs` — the upload is allowed. The
// asymmetry is deliberate: wrongly allowing an upload costs some disk, which an operator can see; wrongly
// refusing one costs a photographer's hand-in and possibly their card. A floor is a safety margin, not an
// authorization.
//
// # Why it warns before it refuses
//
// A 507 in front of a photographer is already a bad day. `adminDiskWarnMultiple` makes the log say so while
// there is still room to act, which is the "check that fires before a batch can fill the volume" task 384
// asks for. It is deliberately a multiple of the floor rather than a second setting: two numbers that have
// to be kept in a sensible relation to each other is two numbers somebody sets inconsistently.

// adminDiskWarnMultiple is how much headroom still earns a warning line, as a multiple of the floor.
//
// Three: enough that a full card's worth of hand-in (order 1 GB, PRD 022 §6) trips it against a 2 GiB floor
// well before the refusal does, and not so much that a healthy machine logs on every upload.
const adminDiskWarnMultiple = 3

// adminDiskVerdict is whether there is room for an upload.
type adminDiskVerdict int

const (
	adminDiskOK adminDiskVerdict = iota
	adminDiskFull
)

// checkAdminDiskFloor reports whether storing `incoming` more bytes keeps the volume above its floor.
//
// `incoming` is the size of what is about to be stored, so the check is against the *result* rather than the
// current state — otherwise an upload one byte above the floor could write a 50 MB file. The same reasoning
// `checkGlimtStorageCeiling` records, and the same mistake it avoids.
func (app *application) checkAdminDiskFloor(incoming int64) adminDiskVerdict {
	floor := app.config.adminDiskFloorBytes
	if floor <= 0 {
		return adminDiskOK
	}

	spacer, ok := app.blobs.(blob.FreeSpacer)
	if !ok {
		// A store with no volume to measure — the in-memory one in tests, or an object store later. See
		// "fails open" above.
		return adminDiskOK
	}

	free, err := spacer.FreeBytes()
	if err != nil {
		app.Logger.Warn("could not measure free space on the blob volume; allowing the upload", "err", err)
		return adminDiskOK
	}

	remaining := int64(free) - incoming
	if remaining < floor {
		// Loud: this is an operator problem happening right now, in front of somebody holding a camera.
		app.Logger.Error("refusing an admin upload: the blob volume is at its floor",
			"freeBytes", free, "incomingBytes", incoming, "floorBytes", floor)
		return adminDiskFull
	}

	if remaining < floor*adminDiskWarnMultiple {
		// The half of task 384 that is a check rather than a refusal: said while there is still room to do
		// something about it.
		app.Logger.Warn("the blob volume is approaching its floor",
			"freeBytes", free, "floorBytes", floor, "warnBelowBytes", floor*adminDiskWarnMultiple)
	}

	return adminDiskOK
}

// writeAdminDiskResponse answers a refused upload, or reports that there was nothing to answer.
//
// Returns true when it has written a response, so the caller can `return` immediately — the same shape as
// `writeGlimtCeilingResponse`.
func (app *application) writeAdminDiskResponse(
	w http.ResponseWriter, r *http.Request, verdict adminDiskVerdict,
) bool {
	if verdict != adminDiskFull {
		return false
	}
	// 507, not 429 and not 400. Nothing the photographer sent is wrong and nothing they can do will help, so
	// the message says whose problem it is and does not invite a retry loop. The same distinction
	// `glimtstorage.go` draws between a member's quota and the server's room.
	app.InsufficientStorageResponse(w, r,
		"Der er ikke plads til flere billeder på serveren. Sig det til en udvikler — "+
			"behold kortet indtil billederne er uploadet.")
	return true
}

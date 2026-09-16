package main

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"strconv"

	"nathejk.dk/internal/reveal"
	"nathejk.dk/internal/scans"
	"nathejk.dk/internal/users"
	"nathejk.dk/nathejk/table/checkpoint"
)

// Cheap "did this change?" versions for the three map datasets (task 269, feeding PRD 017's
// foreground-sync check).
//
// # Why these exist and what they must not become
//
// PRD 017 will have a device ask, on foreground and on an interval, whether its cached checkpoints,
// handouts or scans have changed — without downloading them. `contactsVersionFor` (contacts.go) is the
// reference for that, and these follow it deliberately:
//
//   - a short opaque string, scoped to the caller's permitted set (here: the patrol);
//   - computed from a projection read and hashed over the *data*, never over the rendered payload;
//   - cached keyed by the permitted set rather than by user, so devices sharing a patrol share the work.
//
// Two traps, both learned in contacts.go:
//
//  1. **A wrongly-unstable version is expensive.** Nothing time-varying may enter these hashes — no
//     expiry, no "now", no per-viewer presentation flag. If it did, every poll would mint a new version
//     and every device would refetch everything on every interval, which is the exact cost this endpoint
//     exists to avoid.
//  2. **A wrongly-stable version is silent** — a device that quietly never updates. So each hash covers
//     every field the corresponding payload can expose, and each has a test that it *changes* when its
//     data changes, not merely that it is stable.

// hashFloat writes a float in a canonical shortest-exact form, so an unchanged coordinate always hashes
// the same and a moved one always differs. FormatFloat with -1 precision is round-trippable and stable
// across builds, unlike the raw bit pattern which would be fine too but reads as noise in a diff.
func hashFloat(h io.Writer, f float64) {
	io.WriteString(h, strconv.FormatFloat(f, 'f', -1, 64))
}

func hashFloatPtr(h io.Writer, f *float64) {
	if f == nil {
		// A distinct sentinel, so "no position" hashes differently from any real coordinate — in
		// particular from 0,0, which is itself a real (if unlikely) value.
		io.WriteString(h, "\x00nil")
		return
	}
	hashFloat(h, *f)
}

// checkpointsVersion hashes the revealed set and the next line.
//
// Over the checkpoint rows and the next-checkgroup id, in the order the reveal rule returns them (route
// order, deterministic). `next_checkgroup` is part of it because the client branches on it and an arrow
// moving from one line to the next is a change the device must notice.
func checkpointsVersion(cps []checkpoint.Checkpoint, nextCheckgroup string) string {
	h := sha256.New()
	for _, c := range cps {
		// Every field /api/checkpoints can expose, in a fixed order. A field added to checkpointResponse
		// must be added here too, or a change to it would not invalidate the version.
		io.WriteString(h, string(c.ID))
		io.WriteString(h, "\x00")
		io.WriteString(h, c.Name)
		io.WriteString(h, "\x00")
		io.WriteString(h, string(c.Checkgroup))
		io.WriteString(h, "\x00")
		io.WriteString(h, strconv.Itoa(c.SortOrder))
		io.WriteString(h, "\x00")
		hashFloat(h, c.Lat)
		io.WriteString(h, "\x00")
		hashFloat(h, c.Lng)
		io.WriteString(h, "\x00")
		io.WriteString(h, strconv.FormatInt(c.OpenFromUts, 10))
		io.WriteString(h, "\x00")
		io.WriteString(h, strconv.FormatInt(c.OpenUntilUts, 10))
		io.WriteString(h, "\x00")
		io.WriteString(h, strconv.Itoa(c.OpenDuration))
		io.WriteString(h, "\x1e")
	}
	io.WriteString(h, "\x1d")
	io.WriteString(h, nextCheckgroup)
	return hex.EncodeToString(h.Sum(nil)[:16])
}

// checkpointsVersionFor returns the version of the caller's revealed checkpoints, from a short-lived cache.
//
// Keyed by patrol *and* started-state: the reveal rule is patrol-scoped, and whether the start line is
// still drawn depends on `hasStarted`, so two patrols in the same started-state share an answer while a
// patrol that has just departed gets a fresh one. Not keyed by user — a whole patrol shares its map.
func (app *application) checkpointsVersionFor(viewer users.User) (string, error) {
	if app.models.Maps == nil || viewer.PatrolID == "" {
		return checkpointsVersion(nil, ""), nil
	}

	started := app.hasStarted(viewer.ID)
	key := viewer.PatrolID + "|" + strconv.FormatBool(started)
	if v, ok := app.checkpointsVersions.get(key); ok {
		return v, nil
	}

	revealed, err := app.models.Maps.Revealed(app.config.eventYear, viewer.PatrolID, started)
	if err != nil {
		return "", err
	}
	version := checkpointsVersion(revealed.Checkpoints, string(revealed.NextCheckgroup))
	app.checkpointsVersions.put(key, version)
	return version, nil
}

// handoutsVersion hashes the patrol's map sheets.
//
// Over the handout rows in the order Handouts returns them (oldest first, deterministic). StillHeld is in
// it: a sheet moving from held to afleveret is a change the drawer must reflect, even though nothing else
// about the row moves.
func handoutsVersion(hs []reveal.Handout) string {
	h := sha256.New()
	for _, ho := range hs {
		io.WriteString(h, string(ho.Sheet))
		io.WriteString(h, "\x00")
		io.WriteString(h, ho.Name)
		io.WriteString(h, "\x00")
		io.WriteString(h, string(ho.Format))
		io.WriteString(h, "\x00")
		io.WriteString(h, ho.QrID)
		io.WriteString(h, "\x00")
		io.WriteString(h, strconv.FormatInt(ho.HandedOutUts, 10))
		io.WriteString(h, "\x00")
		io.WriteString(h, strconv.FormatBool(ho.StillHeld))
		io.WriteString(h, "\x1e")
	}
	return hex.EncodeToString(h.Sum(nil)[:16])
}

// handoutsVersionFor returns the version of the caller's handouts, cached by patrol.
func (app *application) handoutsVersionFor(viewer users.User) (string, error) {
	if app.models.Maps == nil || viewer.PatrolID == "" {
		return handoutsVersion(nil), nil
	}

	if v, ok := app.handoutsVersions.get(viewer.PatrolID); ok {
		return v, nil
	}

	handouts, err := app.models.Maps.Handouts(app.config.eventYear, viewer.PatrolID)
	if err != nil {
		return "", err
	}
	version := handoutsVersion(handouts)
	app.handoutsVersions.put(viewer.PatrolID, version)
	return version, nil
}

// scansVersion hashes the patrol's registrations, verdict included.
//
// Over the scan rows in the order ByPatrol returns them (newest first, deterministic). The verdict is
// hashed because a scan that gains an on-time verdict — a `relative` window whose anchor has now been
// scanned — has changed for the drawer even though the scan row itself is untouched.
func scansVersion(ss []scans.Scan) string {
	h := sha256.New()
	for _, s := range ss {
		io.WriteString(h, s.ID)
		io.WriteString(h, "\x00")
		io.WriteString(h, string(s.Kind))
		io.WriteString(h, "\x00")
		io.WriteString(h, s.Label)
		io.WriteString(h, "\x00")
		io.WriteString(h, s.CheckpointID)
		io.WriteString(h, "\x00")
		hashFloatPtr(h, s.Lat)
		io.WriteString(h, "\x00")
		hashFloatPtr(h, s.Lng)
		io.WriteString(h, "\x00")
		io.WriteString(h, strconv.FormatInt(s.ScannedAt.Unix(), 10))
		io.WriteString(h, "\x00")
		if v := s.Verdict; v != nil {
			io.WriteString(h, strconv.FormatBool(v.OnTime))
			io.WriteString(h, ":")
			io.WriteString(h, strconv.Itoa(v.DeltaSeconds))
			io.WriteString(h, ":")
			// Part of the payload, so it has to be part of the hash — the elapsed time can change while the
			// verdict does not (a corrected anchor scan), and a version that missed that would strand a
			// device on a stale duration.
			if v.SpentSeconds != nil {
				io.WriteString(h, strconv.Itoa(*v.SpentSeconds))
			} else {
				io.WriteString(h, "\x00nospent")
			}
		} else {
			io.WriteString(h, "\x00none")
		}
		io.WriteString(h, "\x1e")
	}
	return hex.EncodeToString(h.Sum(nil)[:16])
}

// scansVersionFor returns the version of the caller's registrations, cached by patrol.
func (app *application) scansVersionFor(viewer users.User) (string, error) {
	if viewer.PatrolID == "" {
		return scansVersion(nil), nil
	}

	if v, ok := app.scansVersions.get(viewer.PatrolID); ok {
		return v, nil
	}

	version := scansVersion(app.models.Scans.ByPatrol(viewer.PatrolID))
	app.scansVersions.put(viewer.PatrolID, version)
	return version, nil
}

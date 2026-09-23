package main

import (
	"log/slog"

	"nathejk.dk/internal/scans"
	"nathejk.dk/nathejk/table/person"
	"nathejk.dk/nathejk/table/scan"
)

// scanSourceFor picks the registrations source: the real projection when there is one, the seeded mock
// when there is not.
//
// # Why the mock survives
//
// Running without a database is a supported mode (PRD 008 §5), and the map page is one of the first
// things anyone opens in development. A source that returned nothing there would make the drawer, the
// markers and the arrows unreachable without a broker and a replay — so the fixture stays as the
// no-database fallback, and is the only thing that can produce a bandit catch today (see
// scans.NewProjectionSource).
//
// It is chosen by the **absence of a projection**, not by an environment flag. That matters: a
// misconfigured production deployment gets an empty list and a log line about a missing projection,
// rather than silently serving a plausible evening of invented scans to real patrols. Fixture data
// reaching a race would be worse than no data, because it looks right.
//
// # Why the adapter is here and not in internal/scans
//
// It needs the concrete projection type, and `internal/scans` must not import anything under
// nathejk/table — the seam exists so that dependency runs one way only.
func scanSourceFor(t *scan.Table, people person.Queries, year string, logger *slog.Logger) scans.Source {
	if t == nil {
		logger.Warn("no scan projection: serving mock registrations (development fallback)")
		return scans.NewMockSource()
	}
	return scans.NewProjectionSource(projectionAdapter{t: t}, year, banditterOrNil(people), func(err error) {
		// Reported rather than swallowed. `Source.ByPatrol` cannot return an error — the map must stay
		// usable when registrations fail — so without this a broken projection would be
		// indistinguishable from a patrol that has not scanned anything yet.
		logger.Error("reading patrol registrations", "err", err)
	})
}

// banditterOrNil adapts the person projection to the bandit classifier, preserving nil-ness.
//
// The same nil-interface trap as `mapReadsOrNil`: a nil `person.Queries` assigned to a
// `scans.Banditter` variable is an interface that is *not* nil, so the source would call through it and
// panic. Untyped nil instead, which the source treats as "no classification available" — leaving every
// registration a checkpoint scan, the safe direction.
func banditterOrNil(people person.Queries) scans.Banditter {
	if people == nil {
		return nil
	}
	return banditter{people: people}
}

// banditter resolves the year's banditter from the person projection.
//
// # Why the role and not a team
//
// A bandit is a *person* in this repo's model — `person.Classify` maps the senior population to
// `RoleBandit` — and the scan event gives us a person id. Going via teams would mean a second projection
// to answer a question the person one already answers.
//
// # Why one query, not a lookup per scan
//
// The year's banditter are a few dozen rows and the map refetches the registration list, so a single
// indexed read plus set membership beats one read per registration. `ListByAppRoles` is the same query the
// contacts directory already uses.
//
// # Why it refuses to answer for some years (task 344)
//
// `Classify` maps **the whole senior population** to `RoleBandit`, unconditionally. That is right for a year
// where post personnel sign up as *crew*, and catastrophically wrong for a year where everybody who staffed
// the event signed up as a senior — because then "senior" means "helped out", not "bandit".
//
// 2025 is such a year, measured rather than suspected:
//
//	3,588 scans — 1,280 (35.7%) scanner ids that resolve to nobody in the projection,
//	2,308 (64.3%) that resolve, and **every single one** to appRole=bandit.
//	2,465 people in the year: 1,366 bandit, 974 spejder, 125 gøgler, and **zero** crew,
//	postmandskab, guide or samarit. Zero section slugs.
//
// So classifying by role alone would have told 2,308 registrations they were bandit catches — telling a
// patrol they were caught at every post they visited. There is no corroborating signal to fall back on
// either: `armNumber` is empty for all 2,465 of them, and `checkpersonnel` has **2** shift rows for the
// whole of 2025, so "was this scanner on a post rota?" cannot be asked.
//
// The guard therefore asks whether the year distinguishes crew from seniors **at all**. If it does not, no
// scan is called a bandit catch, and every registration reads as what we actually know: a registration, at a
// time, in a place. 2026 has 160 crew-role people, so it classifies exactly as before.
//
// The asymmetry this protects is the one `kindFor` already records: missing a catch understates the night,
// while inventing one tells a patrol something false that they would act on — and on the public page, that
// they would send to their family.
type banditter struct {
	people person.Queries
}

// crewRoles are the roles whose presence proves a year separates staff from seniors.
//
// Any one of them is enough: they exist only where a crew signup with a recognised section slug happened,
// which is precisely the condition under which "senior" narrows to "bandit".
var crewRoles = []string{person.RoleCrew, person.RolePostmandskab, person.RoleGuide, person.RoleSamarit}

func (b banditter) BanditIDs(year string) (map[string]bool, error) {
	// The capability check first, because its answer can make the second query pointless.
	crew, err := b.people.ListByAppRoles(year, crewRoles)
	if err != nil {
		return nil, err
	}
	if len(crew) == 0 {
		// An undifferentiated year. Nil rather than an error: this is a fact about the event's signup
		// data, not a failure, and `internal/scans` already treats an empty set as "no banditter known"
		// and labels everything a checkpoint visit.
		return nil, nil
	}

	rows, err := b.people.ListByAppRoles(year, []string{person.RoleBandit})
	if err != nil {
		return nil, err
	}
	ids := make(map[string]bool, len(rows))
	for _, p := range rows {
		ids[p.PersonID] = true
	}
	return ids, nil
}

// projectionAdapter converts the projection's rows into the shape internal/scans declares.
//
// A hand-written loop rather than a shared struct, because the two types are deliberately separate: one
// is a row in a read model, the other is a registration a patrol sees. Keeping them apart is what lets
// the projection grow columns — the window and scheme it already carries for the verdict work — without
// widening what the handler is handed.
type projectionAdapter struct {
	t *scan.Table
}

func (a projectionAdapter) ByTeam(year, teamID string) ([]scans.ProjectedScan, error) {
	rows, err := a.t.ByTeam(year, teamID)
	if err != nil {
		return nil, err
	}
	out := make([]scans.ProjectedScan, 0, len(rows))
	for _, r := range rows {
		out = append(out, scans.ProjectedScan{
			QrID:                 r.QrID,
			Uts:                  r.Uts,
			CheckpointID:         r.CheckpointID,
			CheckpointName:       r.CheckpointName,
			ScannerID:            r.ScannerID,
			Lat:                  r.Lat,
			Lng:                  r.Lng,
			CheckgroupID:         r.CheckgroupID,
			Scheme:               r.Scheme,
			RelativeCheckgroupID: r.RelativeCheckgroupID,
			OpenFromUts:          r.OpenFromUts,
			OpenUntilUts:         r.OpenUntilUts,
			OpenDurationMinutes:  r.OpenDuration,
		})
	}
	return out, nil
}

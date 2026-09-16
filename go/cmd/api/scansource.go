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
// nathejk/table \u2014 the seam exists so that dependency runs one way only.
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
type banditter struct {
	people person.Queries
}

func (b banditter) BanditIDs(year string) (map[string]bool, error) {
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
// the projection grow columns \u2014 the window and scheme it already carries for the verdict work \u2014 without
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

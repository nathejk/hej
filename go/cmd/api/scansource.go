package main

import (
	"log/slog"

	"nathejk.dk/internal/scans"
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
func scanSourceFor(t *scan.Table, year string, logger *slog.Logger) scans.Source {
	if t == nil {
		logger.Warn("no scan projection: serving mock registrations (development fallback)")
		return scans.NewMockSource()
	}
	return scans.NewProjectionSource(projectionAdapter{t: t}, year, func(err error) {
		// Reported rather than swallowed. `Source.ByPatrol` cannot return an error \u2014 the map must stay
		// usable when registrations fail \u2014 so without this a broken projection would be
		// indistinguishable from a patrol that has not scanned anything yet.
		logger.Error("reading patrol registrations", "err", err)
	})
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

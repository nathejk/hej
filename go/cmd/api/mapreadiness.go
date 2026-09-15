package main

import (
	"log/slog"

	"nathejk.dk/nathejk/table/kort"
	"nathejk.dk/nathejk/table/scan"
)

// reportMapReadiness logs whether this year's map data can actually reveal anything.
//
// # Why this exists
//
// PRD 016's characteristic failure is **silence**. Every part of it degrades to "nothing to show", which is
// also a legitimate state for the first hour of a race — so a year whose sheets are all in sets with no
// team type, or whose personnel rota was never filled in, produces an app that works perfectly and shows a
// patrol an empty map. Nobody finds out until somebody phones in lost, and by then the cause is three
// services away.
//
// So: two numbers, logged once, in the spirit of `checkpoint.ReportPositionless` — an aggregate rather than
// an event per row, because individual gaps are expected and only the systematic case matters. "12 sheets, 0
// in a patrulje set" is the shape of the disaster.
//
// # Why a log line and not an endpoint
//
// The audience is whoever is setting the event up, reading the service log the evening before. An endpoint
// would need somebody to know to call it, which is the same problem as the silence.
func reportMapReadiness(logger *slog.Logger, sheets *kort.Table, scans *scan.Table, year string) {
	reportSheetReadiness(logger, sheets, year)
	reportScanAttribution(logger, scans, year)
}

// reportSheetReadiness reports how much of the year's map definitions can reach a patrol.
//
// The interesting number is not how many sheets exist but how many are in a set marked `patrulje` — that is
// the filter the handout list and the reveal rule both apply, and a season where nobody marked the set
// reveals **nothing to anybody** while every other number looks healthy.
func reportSheetReadiness(logger *slog.Logger, sheets *kort.Table, year string) {
	if sheets == nil {
		// Already logged as unavailable at construction; saying it twice adds nothing.
		return
	}

	// The counts come from the projection's own read, which reports them through the option installed
	// below — so this function only has to trigger a read and let the sink do the talking.
	if _, err := sheets.PatrolSheets(year); err != nil {
		logger.Error("could not read map sheets to report readiness", "year", year, "err", err)
	}
}

// mapCountsReporter is the sink handed to kort.ReportCounts.
//
// Levels are chosen so the log tells an operator what to *do*: a year with no patrol set is a warning
// because somebody has to go and mark one, while a year with no sheets at all is merely information — early
// in the season that is simply the truth.
func mapCountsReporter(logger *slog.Logger) func(year string, sets, patrolSets, sheets, patrolSheets int) {
	return func(year string, sets, patrolSets, sheets, patrolSheets int) {
		switch {
		case sheets == 0:
			logger.Info("no map sheets defined for this year yet",
				"year", year, "sets", sets)
		case patrolSets == 0:
			logger.Warn("map sheets exist but none are in a set marked for patrols: "+
				"no patrol will be shown any checkpoints",
				"year", year, "sets", sets, "sheets", sheets, "team_type", string(kort.PatrolTeamType))
		case patrolSheets == 0:
			logger.Warn("a patrol map set exists but holds no sheets",
				"year", year, "patrol_sets", patrolSets, "sheets", sheets)
		default:
			logger.Info("map sheets ready",
				"year", year, "sets", sets, "patrol_sets", patrolSets,
				"sheets", sheets, "patrol_sheets", patrolSheets)
		}
	}
}

// reportScanAttribution reports how many scans the personnel rota could not place.
//
// This is the upstream gap that is invisible from inside this repo. A scan carries no checkpoint; it is
// attributed by asking which post its scanner was on shift at. With shifts missing, scans attribute to
// nothing — so post names disappear from the drawer, on-time verdicts disappear, and reveal rule 3 never
// fires. hq's own code carries the same warning about its equivalent.
//
// Reported as a **ratio**, because the count alone is not actionable: 40 unattributed scans is a rounding
// error in a healthy event and a catastrophe in a quiet one.
func reportScanAttribution(logger *slog.Logger, scans *scan.Table, year string) {
	if scans == nil {
		return
	}

	unattributed, total, err := scans.UnattributedCount(year)
	if err != nil {
		logger.Error("could not read scan attribution", "year", year, "err", err)
		return
	}
	reportAttributionCounts(logger, year, unattributed, total)
}

// reportAttributionCounts decides what to say about the ratio.
//
// Split from the read so the judgement is testable without a database — the judgement is the part that can
// be wrong, and it is the part an operator acts on.
func reportAttributionCounts(logger *slog.Logger, year string, unattributed, total int) {
	if total == 0 {
		// Before the race there are no scans, and that is not worth a line: a periodic "everything is
		// fine" trains people to ignore the channel, and so does a startup one.
		return
	}

	// A tenth is far above PRD 016 §9's ≤ 5 % target, and low enough to catch a rota with a few posts
	// missing rather than only a wholly empty one.
	if unattributed*10 > total {
		logger.Warn("many scans could not be attributed to a checkpoint: "+
			"check the postmandskab rota, or patrols will see no post names and no on-time verdicts",
			"year", year, "unattributed", unattributed, "total", total)
		return
	}
	logger.Info("scan attribution",
		"year", year, "unattributed", unattributed, "total", total)
}

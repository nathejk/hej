package scan

import (
	"database/sql"
	"strconv"

	"github.com/jrgensen/cqrs"
)

// Queries is the read API handed to the application.
type Queries interface {
	// ByTeam returns a team's scans, newest first, each attributed to a checkpoint where the rota
	// allows it.
	//
	// Empty is a normal answer: a patrol before its first post, and every user without a patrol.
	ByTeam(year, teamID string) ([]Scan, error)

	// UnattributedCount reports how many of the year's scans could not be attributed to a checkpoint.
	//
	// A diagnostic, not a feature (task 260). The postmandskab rota is fed from outside this repo, and
	// when it is incomplete this feature quietly does nothing — no post names, no verdicts, no
	// reveal by checkgroup. A number is the only way that becomes visible before a patrol phones in.
	UnattributedCount(year string) (unattributed, total int, err error)
}

// Scan is one scan of a team's code.
type Scan struct {
	// QrID and Uts together identify the scan; the event carries no id of its own.
	QrID string
	Uts  int64

	ScannerID string

	// CheckpointID and CheckpointName are set when the scan could be attributed to a post, i.e. when
	// the scanner was on a registered shift there at that moment.
	//
	// **Empty is a normal outcome, not an error**, and the caller must render it as a plain scan
	// rather than hiding it: the scan happened, and a patrol that sees its registration disappear
	// because a shift was not recorded would reasonably conclude the app is broken. No checkpoint also
	// means no on-time verdict — see the scheme fields.
	CheckpointID   string
	CheckpointName string

	// CheckgroupID is the attributed checkpoint's group, for reveal rule 3.
	CheckgroupID string

	// The attributed checkpoint's window and its group's scheme, carried through so the verdict can be
	// computed without a second query (task 265). Zero and "" when unattributed.
	OpenFromUts          int64
	OpenUntilUts         int64
	OpenDuration         int
	Scheme               string
	RelativeCheckgroupID string

	// Lat and Lng are nil when the scan carried no position, or one that will not parse.
	//
	// Nil rather than zero: 0,0 is the Atlantic off Ghana, and the client distinguishes "listed but
	// not plottable" from "plotted at the equator". The same reasoning as the checkpoint projection's
	// treatment of an unset coordinate.
	Lat *float64
	Lng *float64
}

type querier struct {
	db cqrs.Reader
}

// byTeamQuery attributes each of a team's scans to a checkpoint, where the rota allows it.
//
// # The join is the whole point
//
// A scan carries no checkpoint. The only link is who scanned it and when, so a scan counts for a post
// if the scanner was on a registered shift there at that moment. This predicate is copied from hq's
// live implementation (`scansByCheckgroup`) deliberately: the app and the organizers' own screens must
// agree about which post a scan happened at, and about who was on time. Two systems inferring the same
// thing differently is worse than either inference being imperfect.
//
// # Why LEFT JOIN
//
// An unattributable scan must still be listed. It happened — the patrol was there and the post scanned
// their code — and the only thing missing is our ability to say where. An INNER JOIN would silently
// drop it, which turns an upstream rota gap into "the app lost our scan".
//
// # Why the checkpoint's window travels with the scan
//
// So the verdict can be computed without a second round trip per row. `checkgroup` is joined for the
// scheme, because whether the window is absolute or anchored on another group's scan is a property of
// the group, not the post.
//
// Bounds are **inclusive**, matching hq. A shift with a zero window matches nothing, which is how an
// hours-less shift correctly attributes no scans.
const byTeamQuery = `
	SELECT s.qrId, s.uts, s.scannerId, s.latitude, s.longitude,
	       COALESCE(cp.checkpointId, ''), COALESCE(cp.name, ''), COALESCE(cp.checkgroupId, ''),
	       COALESCE(cp.openFromUts, 0), COALESCE(cp.openUntilUts, 0), COALESCE(cp.openDuration, 0),
	       COALESCE(cg.scheme, ''), COALESCE(cg.relativeCheckgroupId, '')
	FROM scan s
	LEFT JOIN checkpersonnel cpn
	       ON cpn.year = s.year AND cpn.userId = s.scannerId
	      AND s.uts >= cpn.startUts AND s.uts <= cpn.endUts
	LEFT JOIN checkpoint cp
	       ON cp.checkpointId = cpn.checkpointId AND cp.year = s.year AND cp.deleted = 0
	LEFT JOIN checkgroup cg
	       ON cg.checkgroupId = cp.checkgroupId AND cg.year = s.year AND cg.deleted = 0
	WHERE s.year = ? AND s.teamId = ?
	ORDER BY s.uts DESC, s.qrId ASC`

// ByTeam returns the team's scans, newest first.
//
// Newest first because that is the order the drawer shows and the order a patrol thinks in: "what did we
// just do". `qrId` breaks ties so two scans in the same second come back stably.
func (q querier) ByTeam(year, teamID string) ([]Scan, error) {
	rows, err := q.db.Query(byTeamQuery, year, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Scan{}
	for rows.Next() {
		var (
			s        Scan
			lat, lng string
		)
		if err := rows.Scan(&s.QrID, &s.Uts, &s.ScannerID, &lat, &lng,
			&s.CheckpointID, &s.CheckpointName, &s.CheckgroupID,
			&s.OpenFromUts, &s.OpenUntilUts, &s.OpenDuration,
			&s.Scheme, &s.RelativeCheckgroupID); err != nil {
			return nil, err
		}
		s.Lat, s.Lng = parsePosition(lat, lng)
		out = append(out, s)
	}
	return out, rows.Err()
}

// parsePosition turns the stored strings into a coordinate, or nothing.
//
// Both must parse and neither may be the 0,0 placeholder. Returning nil rather than a zero pair is the
// point: the client lists a registration with no position but does not plot it, and a scan plotted off
// the coast of Ghana would be worse than one not plotted at all.
//
// A position that will not parse is treated exactly like one that was absent. It is a scan either way,
// and refusing the row over an unparseable string would lose the registration.
func parsePosition(lat, lng string) (*float64, *float64) {
	if lat == "" || lng == "" {
		return nil, nil
	}
	latF, err := strconv.ParseFloat(lat, 64)
	if err != nil {
		return nil, nil
	}
	lngF, err := strconv.ParseFloat(lng, 64)
	if err != nil {
		return nil, nil
	}
	if latF == 0 && lngF == 0 {
		return nil, nil
	}
	return &latF, &lngF
}

// UnattributedCount counts the year's scans that no shift could place.
//
// One query, two numbers, so the caller can log a ratio rather than a bare count — "40 of 900" is
// actionable where "40" is not.
func (q querier) UnattributedCount(year string) (int, int, error) {
	const query = `
		SELECT
			SUM(CASE WHEN cpn.id IS NULL THEN 1 ELSE 0 END) AS unattributed,
			COUNT(*) AS total
		FROM scan s
		LEFT JOIN checkpersonnel cpn
		       ON cpn.year = s.year AND cpn.userId = s.scannerId
		      AND s.uts >= cpn.startUts AND s.uts <= cpn.endUts
		WHERE s.year = ?`

	var unattributed sql.NullInt64
	var total int
	if err := q.db.QueryRow(query, year).Scan(&unattributed, &total); err != nil {
		return 0, 0, err
	}
	return int(unattributed.Int64), total, nil
}

var _ Queries = querier{}

package main

import (
	"errors"
	"time"

	"nathejk.dk/internal/users"
	"nathejk.dk/nathejk/table/glimt"
)

// errRefCheckFailed stands in for a database that cannot answer "is this object shared?".
var errRefCheckFailed = errors.New("ref check failed")

// stubGlimt is a fake glimt.Queries for handler tests.
//
// Holds rows in memory and applies the *same* `Filter.Matches` the real querier's WHERE clause
// encodes, rather than returning everything. A stub that ignored the filter would make every
// visibility test in this package vacuous — the handlers would look correct while the only thing
// actually enforcing visibility (the SQL, or the predicate on the way out) went untested. This is
// the lesson `stubPeople.ListByAppRoles` records for the contacts manifest, applied here.
type stubGlimt struct {
	rows []glimt.Glimt

	// filters records the filter each read was given, so a test can assert that a handler
	// derived a *narrowing* one rather than an unrestricted one.
	filters []glimt.Filter

	// version is what Version returns; err fails every read.
	version string
	err     error

	// moderationCalls counts Moderation reads, which must only ever happen behind the
	// Team-section check.
	moderationCalls int

	// ignoreFilter makes the reads return every row regardless of the filter, simulating a
	// WHERE clause that has drifted from MaySeeGlimt. Used to prove the handler re-checks the
	// predicate on the way out rather than trusting the query.
	ignoreFilter bool

	// refsErr fails RefsUsedElsewhere, so a test can assert that an unanswerable
	// "is this shared?" question leaves the objects in place.
	refsErr error
}

func (s *stubGlimt) Feed(_ string, f glimt.Filter, limit, offset int) ([]glimt.Glimt, error) {
	s.filters = append(s.filters, f)
	if s.err != nil {
		return nil, s.err
	}
	return page(s.matching(f), limit, offset), nil
}

func (s *stubGlimt) ByHold(_ string, teamNumber string, f glimt.Filter, limit, offset int) ([]glimt.Glimt, error) {
	s.filters = append(s.filters, f)
	if s.err != nil {
		return nil, s.err
	}
	if teamNumber == "" {
		return nil, nil
	}
	var out []glimt.Glimt
	for _, g := range s.matching(f) {
		if g.TeamNumber == teamNumber {
			out = append(out, g)
		}
	}
	return page(out, limit, offset), nil
}

func (s *stubGlimt) Get(_ string, glimtID string) (glimt.Glimt, bool, error) {
	if s.err != nil {
		return glimt.Glimt{}, false, s.err
	}
	for _, g := range s.rows {
		// Unfiltered, like the real Get: it backs DELETE authorization and the media handler,
		// which need the row in order to decide.
		if g.GlimtID == glimtID {
			return g, true, nil
		}
	}
	return glimt.Glimt{}, false, nil
}

func (s *stubGlimt) Holds(_ string, f glimt.Filter) ([]glimt.Hold, error) {
	s.filters = append(s.filters, f)
	if s.err != nil {
		return nil, s.err
	}
	byNumber := map[string]*glimt.Hold{}
	var order []string
	for _, g := range s.matching(f) {
		if g.TeamNumber == "" {
			continue
		}
		h, seen := byNumber[g.TeamNumber]
		if !seen {
			byNumber[g.TeamNumber] = &glimt.Hold{
				TeamNumber: g.TeamNumber,
				TeamName:   g.TeamName,
				Group:      g.AuthorGroup,
				Count:      1,
				LatestAt:   g.CreatedAt,
			}
			order = append(order, g.TeamNumber)
			continue
		}
		h.Count++
		if g.CreatedAt.After(h.LatestAt) {
			h.LatestAt = g.CreatedAt
		}
	}
	out := make([]glimt.Hold, 0, len(order))
	for _, n := range order {
		out = append(out, *byNumber[n])
	}
	return out, nil
}

func (s *stubGlimt) Moderation(_ string, limit, offset int) ([]glimt.Glimt, error) {
	s.moderationCalls++
	if s.err != nil {
		return nil, s.err
	}
	// Every row, every scope, including hidden — the real query takes no filter either, which
	// is why the handler's Team-section check is the only thing guarding it.
	return page(append([]glimt.Glimt(nil), s.rows...), limit, offset), nil
}

func (s *stubGlimt) PublicFeed(_ string, notBefore time.Time, limit, offset int) ([]glimt.Glimt, error) {
	if s.err != nil {
		return nil, s.err
	}
	var out []glimt.Glimt
	for _, g := range s.rows {
		if g.Audience != glimt.AudiencePublic || g.HiddenAt != nil {
			continue
		}
		// The public retention cutoff, applied as the real query does.
		if !notBefore.IsZero() && g.CreatedAt.Before(notBefore) {
			continue
		}
		out = append(out, g)
	}
	return page(out, limit, offset), nil
}

func (s *stubGlimt) Expired(_ string, before time.Time, limit int) ([]glimt.Expired, error) {
	if s.err != nil {
		return nil, s.err
	}
	var out []glimt.Expired
	for _, g := range s.rows {
		if g.CreatedAt.Before(before) {
			e := glimt.Expired{GlimtID: g.GlimtID}
			for _, m := range g.Media {
				e.Refs = append(e.Refs, m.Ref)
				if m.ThumbRef != "" {
					e.Refs = append(e.Refs, m.ThumbRef)
				}
			}
			out = append(out, e)
		}
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// RefsUsedElsewhere reports which refs another surviving glimt still references.
//
// Implemented properly rather than returning nil, because the property it protects — that deleting
// one glimt must not blank another that shares bytes — is exactly what a test needs to exercise, and
// a stub that always said "nothing is shared" would make that test pass while the behaviour was
// absent.
func (s *stubGlimt) RefsUsedElsewhere(_ string, glimtID string, refs []string) (map[string]bool, error) {
	if s.refsErr != nil {
		return nil, s.refsErr
	}
	wanted := map[string]bool{}
	for _, r := range refs {
		wanted[r] = true
	}
	inUse := map[string]bool{}
	for _, g := range s.rows {
		if g.GlimtID == glimtID {
			continue
		}
		for _, m := range g.Media {
			if wanted[m.Ref] {
				inUse[m.Ref] = true
			}
			if m.ThumbRef != "" && wanted[m.ThumbRef] {
				inUse[m.ThumbRef] = true
			}
		}
	}
	return inUse, nil
}

func (s *stubGlimt) Version(_ string, f glimt.Filter) (string, error) {
	s.filters = append(s.filters, f)
	if s.err != nil {
		return "", s.err
	}
	if s.version != "" {
		return s.version, nil
	}
	return "stub-version", nil
}

// matching applies the filter, in insertion order, as Feed does.
//
// Converts the query filter back into the `users` one and uses `Matches` — the same predicate the
// real WHERE clause encodes, and the one `internal/users` proves agrees with `MaySeeGlimt`. Writing
// a third implementation of the rule here would defeat the purpose of the tests using it.
//
// Rows are never "deleted" in this stub: the real queries all carry `deleted = 0`, so a deleted row
// is simply absent from what a querier returns, and a test wanting that state removes the row.
func (s *stubGlimt) matching(f glimt.Filter) []glimt.Glimt {
	uf := users.GlimtFeedFilter{
		Denied:        f.Denied,
		Unrestricted:  f.Unrestricted,
		PersonID:      f.PersonID,
		Group:         users.GlimtGroup(f.Group),
		IncludeHidden: f.Unrestricted,
	}
	var out []glimt.Glimt
	for _, g := range s.rows {
		if !s.ignoreFilter && !uf.Matches(glimtSubjectOf(g)) {
			continue
		}
		out = append(out, g)
	}
	return out
}

func page(rows []glimt.Glimt, limit, offset int) []glimt.Glimt {
	if offset < 0 {
		offset = 0
	}
	if offset >= len(rows) {
		return nil
	}
	rows = rows[offset:]
	if limit <= 0 {
		limit = 20
	}
	if len(rows) > limit {
		rows = rows[:limit]
	}
	return rows
}

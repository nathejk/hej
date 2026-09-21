package main

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"nathejk.dk/nathejk/table/person"
)

// Scan-kind classification, and the year it must refuse to classify (PRD 011 §8, task 344).
//
// # What these tests are really about
//
// `person.Classify` maps the **whole senior population** to `RoleBandit`. That is correct for a year where
// post personnel sign up as crew, and wrong for a year where everyone who helped signed up as a senior.
// Measured on 2025: 2,308 of 3,588 scans would have been labelled bandit catches — every registration at
// every post — because the year has 1,366 "bandit" people and zero crew.
//
// The rule is therefore not "trust the role" but "trust the role only where the year separates staff from
// seniors". These tests pin both halves, because the failure is silent and reads as a feature.

// roleQueries answers ListByAppRoles from a fixed table and records what it was asked.
type roleQueries struct {
	byRole map[string][]person.Person
	asked  [][]string
	err    error
}

func (q *roleQueries) ListByAppRoles(_ string, roles []string) ([]person.Person, error) {
	q.asked = append(q.asked, roles)
	if q.err != nil {
		return nil, q.err
	}
	var out []person.Person
	for _, role := range roles {
		out = append(out, q.byRole[role]...)
	}
	return out, nil
}

// The rest of person.Queries, unused here.
func (q *roleQueries) Lookup(string, string) ([]person.Person, error) { return nil, nil }
func (q *roleQueries) Get(string, string) (person.Person, bool, error) {
	return person.Person{}, false, nil
}
func (q *roleQueries) ListPatrolByNumber(string, string) ([]person.Person, error) { return nil, nil }
func (q *roleQueries) TrackMembers(string, string) ([]person.TrackMember, error)  { return nil, nil }
func (q *roleQueries) ExpiredPortraits(string, time.Time, int) ([]person.ExpiredPortrait, error) {
	return nil, nil
}

// **The 2025 shape: no crew at all.** Every helper is a senior, so "senior" cannot narrow to "bandit" and
// nothing may be called a catch.
func TestAnUndifferentiatedYearIdentifiesNoBanditter(t *testing.T) {
	q := &roleQueries{byRole: map[string][]person.Person{
		// 1,366 of these in the real 2025 data; three is enough to make the point.
		person.RoleBandit: {{PersonID: "s-1"}, {PersonID: "s-2"}, {PersonID: "s-3"}},
	}}

	ids, err := banditter{people: q}.BanditIDs("2025")
	if err != nil {
		t.Fatalf("BanditIDs: %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("want no banditter identified in a year with no crew, got %v", ids)
	}

	// And it must not have wasted a query asking for banditter it was never going to trust.
	if len(q.asked) != 1 {
		t.Errorf("want one query, got %v", q.asked)
	}
	if !reflect.DeepEqual(q.asked[0], crewRoles) {
		t.Errorf("the first query must be the capability check, got %v", q.asked[0])
	}
}

// **The 2026 shape: crew exists**, so the senior population means what the classifier says it means.
func TestAYearWithCrewClassifiesBanditterAsBefore(t *testing.T) {
	q := &roleQueries{byRole: map[string][]person.Person{
		person.RolePostmandskab: {{PersonID: "c-1"}},
		person.RoleBandit:       {{PersonID: "b-1"}, {PersonID: "b-2"}},
	}}

	ids, err := banditter{people: q}.BanditIDs("2026")
	if err != nil {
		t.Fatalf("BanditIDs: %v", err)
	}
	if !ids["b-1"] || !ids["b-2"] {
		t.Errorf("want both banditter identified, got %v", ids)
	}
	if ids["c-1"] {
		t.Error("a post crew member is not a bandit")
	}
}

// A single crew member is enough. The check asks whether the year *distinguishes* the populations at all, not
// whether it is fully staffed — one recognised crew signup proves seniors are not the residue.
func TestOneCrewMemberIsEnoughToTrustTheClassification(t *testing.T) {
	q := &roleQueries{byRole: map[string][]person.Person{
		person.RoleSamarit: {{PersonID: "sam-1"}},
		person.RoleBandit:  {{PersonID: "b-1"}},
	}}

	ids, err := banditter{people: q}.BanditIDs("2026")
	if err != nil {
		t.Fatalf("BanditIDs: %v", err)
	}
	if !ids["b-1"] {
		t.Errorf("want the bandit identified, got %v", ids)
	}
}

// A failed read is an error, not an empty set. `internal/scans` treats a failure as "no banditter known" and
// carries on — which is the right degradation — but it must be told, because silently returning nothing here
// would make a projection outage indistinguishable from an undifferentiated year.
func TestAFailedRoleReadIsReported(t *testing.T) {
	q := &roleQueries{err: errors.New("db is down")}

	if _, err := (banditter{people: q}).BanditIDs("2026"); err == nil {
		t.Error("a failed read must be reported rather than read as \"no banditter\"")
	}
}

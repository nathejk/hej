package person

import "testing"

func TestClassifyByPopulation(t *testing.T) {
	tests := []struct {
		name       string
		population Population
		slug       string
		wantRole   string
		wantOK     bool
	}{
		{"spejder", PopulationSpejder, "", RoleSpejder, true},
		{"senior becomes bandit", PopulationSenior, "", RoleBandit, true},
		{"gøgler", PopulationGoegler, "", RoleGoegler, true},
		{"crew postmandskab", PopulationCrew, "postmandskab", RolePostmandskab, true},
		{"crew guide", PopulationCrew, "guide", RoleGuide, true},
		{"crew samarit", PopulationCrew, "samarit", RoleSamarit, true},

		// A slug is a hand-typed admin field, so case and padding are noise.
		{"slug case is folded", PopulationCrew, "Samarit", RoleSamarit, true},
		{"slug whitespace is trimmed", PopulationCrew, "  guide  ", RoleGuide, true},
		{"slug mixed case and padding", PopulationCrew, " POSTMANDSKAB ", RolePostmandskab, true},

		// The two cases that must never lock anyone out.
		{"unassigned crew is generic, not an error", PopulationCrew, "", RoleCrew, true},
		{"unmapped slug falls back and is reported", PopulationCrew, "kagebord", RoleCrew, false},

		// Defensive: an unknown population gets the least-privileged role, not a guess.
		{"unknown population", PopulationUnknown, "", RoleCrew, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			role, ok := Classify(tc.population, tc.slug)
			if role != tc.wantRole {
				t.Errorf("role = %q, want %q", role, tc.wantRole)
			}
			if ok != tc.wantOK {
				t.Errorf("ok = %v, want %v", ok, tc.wantOK)
			}
		})
	}
}

// The distinction the whole fallback rests on: "no section yet" is a normal upstream
// state (crewmember has an Unassigned filter) and must not be reported as a data
// problem, whereas an unrecognised slug must be, or the map silently rots as
// organizers rename sections.
func TestUnassignedIsNotReportedButUnmappedIs(t *testing.T) {
	if _, ok := ClassifyCrew(""); !ok {
		t.Error("an unassigned crew member should not be reported as an unmapped slug")
	}
	if _, ok := ClassifyCrew("   "); !ok {
		t.Error("whitespace-only should be treated as unassigned, not unmapped")
	}
	if _, ok := ClassifyCrew("something-an-organizer-invented"); ok {
		t.Error("an unrecognised slug must be reported so the map can be extended")
	}
}

// Every mapped slug must resolve to exactly the role the map declares, and must be
// reported as *known*. The original version of this test also asserted that no entry
// resolves to RoleCrew, on the reasoning that a typo in the map would silently demote a
// real samarit.
//
// That assertion was retired in task 078, when the map gained a second kind of entry:
// real sections that grant nothing extra (kitchen, HQ, PR, ...) are now listed
// explicitly as RoleCrew, so that *absence* from the map means one specific thing —
// "nobody has classified this section yet" — and the unmapped-slug warning becomes
// signal instead of firing on every replay for three well-known sections.
//
// The property the retired assertion protected is preserved below by
// TestCapabilitySlugsDoNotResolveToTheFallback, which checks the entries that are
// *supposed* to grant something.
func TestEveryMappedSlugYieldsItsDeclaredRole(t *testing.T) {
	for slug, want := range crewFunctionBySlug {
		role, ok := ClassifyCrew(slug)
		if !ok {
			t.Errorf("slug %q is in the map but was reported unmapped", slug)
		}
		if role != want {
			t.Errorf("slug %q: role = %q, want %q", slug, role, want)
		}
	}
}

// A slug that is supposed to grant a capability must not resolve to the least-privileged
// fallback. A typo in one of these keys fails safe but would take the SOS page away from
// a samarit who needs it, so it must not be able to hide.
//
// Listed explicitly rather than derived from the map, because deriving it from the same
// data it is checking is how this class of bug survives.
func TestCapabilitySlugsDoNotResolveToTheFallback(t *testing.T) {
	capabilities := map[string]string{
		"postmandskab": RolePostmandskab,
		"postmand":     RolePostmandskab,
		"post":         RolePostmandskab,
		"poster":       RolePostmandskab,
		"postmandskb":  RolePostmandskab,
		"guide":        RoleGuide,
		"guides":       RoleGuide,
		"guider":       RoleGuide,
		"samarit":      RoleSamarit,
		"samaritter":   RoleSamarit,
		"førstehjælp":  RoleSamarit,
	}

	for slug, want := range capabilities {
		role, ok := ClassifyCrew(slug)
		if !ok {
			t.Errorf("capability slug %q is not in the map at all", slug)
			continue
		}
		if role == RoleCrew {
			t.Errorf("capability slug %q resolves to the least-privileged fallback", slug)
		}
		if role != want {
			t.Errorf("capability slug %q: role = %q, want %q", slug, role, want)
		}
	}
}

// The whole 2026 section tree, read off the live stream, must be classified — otherwise
// the unmapped warning fires during a real event and nobody can tell it apart from a
// section that genuinely needs attention.
func TestTheRealSectionTreeIsFullyClassified(t *testing.T) {
	// Verified present on the broker on 2026-08-26 (task 078).
	for _, slug := range []string{
		"bandit", "goeglerledelse", "guides", "hoensegaard", "hq", "koekken",
		"noedtelefon", "postmand", "postmandskab", "pr", "rover", "samarit", "team",
	} {
		if _, ok := ClassifyCrew(slug); !ok {
			t.Errorf("real section %q is unclassified; the replay will warn about it", slug)
		}
	}
}

// The map keys must already be in normalized form, or a lookup can never match them.
func TestMapKeysAreNormalized(t *testing.T) {
	for slug := range crewFunctionBySlug {
		if got := normalizeSlug(slug); got != slug {
			t.Errorf("map key %q is not normalized (want %q) — it can never be matched", slug, got)
		}
	}
}

// Only spejder have a guardian number. Pinned because PRD 005's confirmation step is
// spejder-only on the strength of this, and PRD 003 renders "not applicable" for
// everyone else.
func TestHasGuardianPhone(t *testing.T) {
	if !HasGuardianPhone(PopulationSpejder) {
		t.Error("spejder must have a guardian phone")
	}
	for _, p := range []Population{PopulationSenior, PopulationCrew, PopulationGoegler, PopulationUnknown} {
		if HasGuardianPhone(p) {
			t.Errorf("population %v must not report a guardian phone", p)
		}
	}
}

// The role strings are shared with users.Role and the frontend union. If one drifts,
// a login returns a role the router guard does not recognise.
func TestRoleStringsMatchTheAppEnum(t *testing.T) {
	want := map[string]string{
		RoleSpejder:      "spejder",
		RoleBandit:       "bandit",
		RolePostmandskab: "postmandskab",
		RoleGuide:        "guide",
		RoleSamarit:      "samarit",
		RoleGoegler:      "gøgler",
		RoleCrew:         "crew",
	}
	for got, expected := range want {
		if got != expected {
			t.Errorf("role string changed: got %q, want %q", got, expected)
		}
	}
}

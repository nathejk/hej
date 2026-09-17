package users

import "testing"

// Glimt visibility tests (task 299).
//
// The property worth protecting here is the one a unit test can actually pin down: a
// `group`-scoped glimt must not cross a group boundary, in either direction. Everything
// else in this file exists to stop that rule being weakened by accident — by a new role
// landing in no group, by an unknown audience defaulting open, or by a moderator branch
// migrating out of the predicate into its callers.

func TestGlimtGroupForCoversEveryRole(t *testing.T) {
	// Enumerates AllRoles rather than a local list, so a role added to the app without a
	// group fails here instead of silently failing closed in production.
	want := map[Role]GlimtGroup{
		RoleSpejder:      GlimtGroupSpejder,
		RoleBandit:       GlimtGroupBandit,
		RolePostmandskab: GlimtGroupCrew,
		RoleGuide:        GlimtGroupCrew,
		RoleSamarit:      GlimtGroupCrew,
		RoleGoegler:      GlimtGroupCrew,
		RoleCrew:         GlimtGroupCrew,
	}
	for _, r := range AllRoles {
		expected, listed := want[r]
		if !listed {
			t.Fatalf("role %q has no expected glimt group — add it to this test and to GlimtGroupFor", r)
		}
		got, ok := GlimtGroupFor(r)
		if !ok {
			t.Errorf("GlimtGroupFor(%q) = not ok, want %q", r, expected)
			continue
		}
		if got != expected {
			t.Errorf("GlimtGroupFor(%q) = %q, want %q", r, got, expected)
		}
	}
}

func TestGlimtGroupForUnknownRole(t *testing.T) {
	// A role read back from a session or a stored row is untrusted. It must not land in
	// a group, because the zero value would be compared against a stored group string.
	if g, ok := GlimtGroupFor(Role("hjælper")); ok {
		t.Errorf("GlimtGroupFor(unknown) = %q, ok — want not ok", g)
	}
}

func TestGlimtGroupAndAudienceValid(t *testing.T) {
	for _, g := range AllGlimtGroups {
		if !g.Valid() {
			t.Errorf("group %q not Valid", g)
		}
	}
	if GlimtGroup("").Valid() || GlimtGroup("hq").Valid() {
		t.Error("unknown group reported Valid")
	}
	for _, a := range AllGlimtAudiences {
		if !a.Valid() {
			t.Errorf("audience %q not Valid", a)
		}
	}
	if GlimtAudience("").Valid() || GlimtAudience("everyone").Valid() {
		t.Error("unknown audience reported Valid")
	}
	// The order is load-bearing: the composer offers them in this order and defaults to
	// the first, so the narrowest must stay first (PRD 019 §6).
	if AllGlimtAudiences[0] != GlimtAudienceGroup {
		t.Errorf("AllGlimtAudiences[0] = %q, want the narrowest (%q) first", AllGlimtAudiences[0], GlimtAudienceGroup)
	}
}

func TestMaySeeGlimt(t *testing.T) {
	const (
		author   = "p-author"
		stranger = "p-stranger"
	)
	spejderGlimt := func(a GlimtAudience) GlimtSubject {
		return GlimtSubject{AuthorPersonID: author, AuthorGroup: GlimtGroupSpejder, Audience: a}
	}

	cases := []struct {
		name   string
		viewer GlimtViewer
		g      GlimtSubject
		want   bool
	}{
		{
			name:   "group scope, same group",
			viewer: GlimtViewer{PersonID: stranger, Role: RoleSpejder},
			g:      spejderGlimt(GlimtAudienceGroup),
			want:   true,
		},
		{
			// The rule this whole file exists for.
			name:   "group scope, bandit cannot see a spejder group glimt",
			viewer: GlimtViewer{PersonID: stranger, Role: RoleBandit},
			g:      spejderGlimt(GlimtAudienceGroup),
			want:   false,
		},
		{
			name:   "group scope, crew cannot see a spejder group glimt",
			viewer: GlimtViewer{PersonID: stranger, Role: RoleSamarit},
			g:      spejderGlimt(GlimtAudienceGroup),
			want:   false,
		},
		{
			// And symmetrically — a spejder must not read the crew's in-jokes.
			name:   "group scope, spejder cannot see a crew group glimt",
			viewer: GlimtViewer{PersonID: stranger, Role: RoleSpejder},
			g:      GlimtSubject{AuthorPersonID: author, AuthorGroup: GlimtGroupCrew, Audience: GlimtAudienceGroup},
			want:   false,
		},
		{
			// gøgler and crew share a bucket here even though the directory keeps
			// them apart. If that ever changes, this is the test that says so.
			name:   "group scope, gøgler sees a crew group glimt",
			viewer: GlimtViewer{PersonID: stranger, Role: RoleGoegler},
			g:      GlimtSubject{AuthorPersonID: author, AuthorGroup: GlimtGroupCrew, Audience: GlimtAudienceGroup},
			want:   true,
		},
		{
			name:   "nathejk scope crosses groups",
			viewer: GlimtViewer{PersonID: stranger, Role: RoleBandit},
			g:      spejderGlimt(GlimtAudienceNathejk),
			want:   true,
		},
		{
			name:   "public scope crosses groups",
			viewer: GlimtViewer{PersonID: stranger, Role: RoleCrew},
			g:      spejderGlimt(GlimtAudiencePublic),
			want:   true,
		},
		{
			name:   "author sees own group glimt from another group's perspective",
			viewer: GlimtViewer{PersonID: author, Role: RoleSpejder},
			g:      spejderGlimt(GlimtAudienceGroup),
			want:   true,
		},
		{
			name:   "hidden glimt is invisible to a member of the right group",
			viewer: GlimtViewer{PersonID: stranger, Role: RoleSpejder},
			g:      GlimtSubject{AuthorPersonID: author, AuthorGroup: GlimtGroupSpejder, Audience: GlimtAudienceGroup, Hidden: true},
			want:   false,
		},
		{
			// Hiding beats every audience, including the widest — that is the point
			// of it, since the public scope has no approval queue in front of it.
			name:   "hidden public glimt is invisible to a member",
			viewer: GlimtViewer{PersonID: stranger, Role: RoleBandit},
			g:      GlimtSubject{AuthorPersonID: author, AuthorGroup: GlimtGroupSpejder, Audience: GlimtAudiencePublic, Hidden: true},
			want:   false,
		},
		{
			name:   "author still sees their own hidden glimt",
			viewer: GlimtViewer{PersonID: author, Role: RoleSpejder},
			g:      GlimtSubject{AuthorPersonID: author, AuthorGroup: GlimtGroupSpejder, Audience: GlimtAudienceGroup, Hidden: true},
			want:   true,
		},
		{
			name:   "moderator sees a group glimt from a group they are not in",
			viewer: GlimtViewer{PersonID: stranger, Role: RoleCrew, IsModerator: true},
			g:      spejderGlimt(GlimtAudienceGroup),
			want:   true,
		},
		{
			name:   "moderator sees hidden glimt",
			viewer: GlimtViewer{PersonID: stranger, Role: RoleCrew, IsModerator: true},
			g:      GlimtSubject{AuthorPersonID: author, AuthorGroup: GlimtGroupSpejder, Audience: GlimtAudienceGroup, Hidden: true},
			want:   true,
		},
		{
			// A moderator is a person with an assignment, not a role: the override
			// must not depend on which role they hold.
			name:   "moderator override is independent of role",
			viewer: GlimtViewer{PersonID: stranger, Role: RoleBandit, IsModerator: true},
			g:      GlimtSubject{AuthorPersonID: author, AuthorGroup: GlimtGroupCrew, Audience: GlimtAudienceGroup},
			want:   true,
		},
		{
			name:   "unknown role sees nothing, not even public",
			viewer: GlimtViewer{PersonID: stranger, Role: Role("hjælper")},
			g:      spejderGlimt(GlimtAudiencePublic),
			want:   false,
		},
		{
			name:   "unknown audience fails closed",
			viewer: GlimtViewer{PersonID: stranger, Role: RoleSpejder},
			g:      GlimtSubject{AuthorPersonID: author, AuthorGroup: GlimtGroupSpejder, Audience: GlimtAudience("everyone")},
			want:   false,
		},
		{
			// A glimt with no author recorded is a data fault. An unidentified caller
			// must not be treated as its owner and handed a hidden photo.
			name:   "empty ids do not make a viewer the author",
			viewer: GlimtViewer{PersonID: "", Role: RoleSpejder},
			g:      GlimtSubject{AuthorPersonID: "", AuthorGroup: GlimtGroupCrew, Audience: GlimtAudienceGroup, Hidden: true},
			want:   false,
		},
		{
			// An author's group that failed to parse must not match a viewer whose
			// group also failed to parse.
			name:   "unknown author group blocks the group scope",
			viewer: GlimtViewer{PersonID: stranger, Role: RoleSpejder},
			g:      GlimtSubject{AuthorPersonID: author, AuthorGroup: GlimtGroup("patrulje"), Audience: GlimtAudienceGroup},
			want:   false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := MaySeeGlimt(tc.viewer, tc.g); got != tc.want {
				t.Errorf("MaySeeGlimt() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestGlimtFeedFilterAgreesWithPredicate is the test that keeps the query filter honest.
//
// The filter is a second encoding of the same rule, which is a thing worth having (a feed
// cannot call the predicate before it has rows) and a thing worth distrusting. Rather than
// review the two by eye, enumerate every combination of viewer and subject and assert they
// agree — so the next person to optimise the WHERE clause finds out immediately.
func TestGlimtFeedFilterAgreesWithPredicate(t *testing.T) {
	viewers := []GlimtViewer{}
	for _, r := range AllRoles {
		viewers = append(viewers,
			GlimtViewer{PersonID: "p-viewer", Role: r},
			GlimtViewer{PersonID: "p-viewer", Role: r, IsModerator: true},
		)
	}
	viewers = append(viewers, GlimtViewer{PersonID: "p-viewer", Role: Role("hjælper")})

	subjects := []GlimtSubject{}
	for _, g := range AllGlimtGroups {
		for _, a := range AllGlimtAudiences {
			for _, hidden := range []bool{false, true} {
				for _, author := range []string{"p-viewer", "p-other"} {
					subjects = append(subjects, GlimtSubject{
						AuthorPersonID: author,
						AuthorGroup:    g,
						Audience:       a,
						Hidden:         hidden,
					})
				}
			}
		}
	}

	for _, v := range viewers {
		filter := GlimtFeedFilterFor(v)
		for _, s := range subjects {
			predicate := MaySeeGlimt(v, s)
			matched := filter.Matches(s)
			if predicate != matched {
				t.Errorf("disagreement for viewer %+v subject %+v: MaySeeGlimt=%v filter.Matches=%v",
					v, s, predicate, matched)
			}
		}
	}
}

func TestGlimtFeedFilterForUnknownRoleIsNarrow(t *testing.T) {
	f := GlimtFeedFilterFor(GlimtViewer{PersonID: "p", Role: Role("hj\u00e6lper")})
	if !f.Denied {
		t.Error("unknown role produced a filter that is not Denied")
	}
	if f.Unrestricted {
		t.Error("unknown role produced an unrestricted filter")
	}
	if f.Group != "" {
		t.Errorf("unknown role landed in group %q", f.Group)
	}
	if f.IncludeHidden {
		t.Error("unknown role may include hidden rows")
	}
	// Not even its own rows: a role we cannot recognise is a session we cannot trust.
	if f.Matches(GlimtSubject{AuthorPersonID: "p", AuthorGroup: GlimtGroupSpejder, Audience: GlimtAudiencePublic}) {
		t.Error("Denied filter matched a row")
	}
}

func TestGlimtFeedFilterForModeratorIsUnrestricted(t *testing.T) {
	f := GlimtFeedFilterFor(GlimtViewer{PersonID: "p", Role: RoleCrew, IsModerator: true})
	if !f.Unrestricted || !f.IncludeHidden {
		t.Errorf("moderator filter = %+v, want unrestricted and including hidden", f)
	}
}

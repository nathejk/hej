package glimt

import (
	"regexp"
	"strings"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

func glimtRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"glimtId", "year", "authorPersonId", "authorGroup", "teamNumber", "teamName",
		"audience", "caption", "createdAt", "mediaCount", "hiddenAt", "hiddenBy", "reportCount",
	})
}

func TestFeedIsNewestFirstAndCarriesItsMedia(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	created := time.Date(2026, 9, 17, 21, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta("FROM glimt g")).
		WillReturnRows(glimtRows().
			AddRow("g-2", "2026", "p-1", "spejder", "42", "Ørnene", "group", "sent", created, 1, nil, "", 0).
			AddRow("g-1", "2026", "p-2", "spejder", "43", "Ulvene", "nathejk", "tidligt", created, 2, nil, "", 0))
	mock.ExpectQuery(regexp.QuoteMeta("FROM glimt_media")).
		WillReturnRows(sqlmock.NewRows([]string{
			"glimtId", "ordinal", "blobRef", "thumbRef", "kind", "contentType", "bytes", "width", "height", "durationMs",
		}).
			AddRow("g-1", 0, refA, refB, "image", "image/jpeg", 1000, 1024, 768, 0).
			AddRow("g-1", 1, refC, "", "video", "video/mp4", 5000, 1920, 1080, 12000).
			AddRow("g-2", 0, refA, refB, "image", "image/jpeg", 900, 800, 600, 0))

	got, err := querier{db: db}.Feed("2026", Filter{PersonID: "p-1", Group: "spejder"}, 20, 0)
	if err != nil {
		t.Fatalf("Feed: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 glimt, got %d", len(got))
	}
	// Media are attached to the right parent, in ordinal order.
	if len(got[1].Media) != 2 || got[1].Media[0].Ordinal != 0 || got[1].Media[1].Kind != MediaKindVideo {
		t.Errorf("media for g-1: %+v", got[1].Media)
	}
	if len(got[0].Media) != 1 {
		t.Errorf("media for g-2: %+v", got[0].Media)
	}
	if got[1].Media[1].DurationMs != 12000 {
		t.Errorf("video duration lost: %+v", got[1].Media[1])
	}
}

func TestFeedOrderAndHoldOrderAreOpposite(t *testing.T) {
	// Not an inconsistency: during the race newest-first is right, and afterwards a hold's
	// collection is somebody reliving an evening in order (PRD 019 §0a.1). Asserted on the
	// SQL because it is the kind of thing a later "tidy-up" would unify.
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery("ORDER BY g.createdAt DESC").WillReturnRows(glimtRows())
	if _, err := (querier{db: db}).Feed("2026", Filter{PersonID: "p-1"}, 20, 0); err != nil {
		t.Fatalf("Feed: %v", err)
	}

	mock.ExpectQuery("ORDER BY g.createdAt ASC").WillReturnRows(glimtRows())
	if _, err := (querier{db: db}).ByHold("2026", "42", Filter{PersonID: "p-1"}, 20, 0); err != nil {
		t.Fatalf("ByHold: %v", err)
	}
}

func TestDeniedFilterNeverReachesTheDatabase(t *testing.T) {
	// The fail-closed path must not depend on a WHERE clause being assembled correctly. No
	// expectations are registered, so any query at all fails the test.
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	q := querier{db: db}
	f := Filter{Denied: true}

	if got, err := q.Feed("2026", f, 20, 0); err != nil || got != nil {
		t.Errorf("Feed = %v, %v; want nil, nil", got, err)
	}
	if got, err := q.ByHold("2026", "42", f, 20, 0); err != nil || got != nil {
		t.Errorf("ByHold = %v, %v; want nil, nil", got, err)
	}
	if got, err := q.Holds("2026", f); err != nil || got != nil {
		t.Errorf("Holds = %v, %v; want nil, nil", got, err)
	}
	if got, err := q.Version("2026", f); err != nil || got != "" {
		t.Errorf("Version = %q, %v; want \"\", nil", got, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet/unexpected: %v", err)
	}
}

func TestWhereClauseShapes(t *testing.T) {
	// The clause is the SQL half of the visibility rule, so its shape is worth pinning.
	// internal/users asserts it agrees with MaySeeGlimt; this asserts it says what it means.
	t.Run("denied is 0 and never 1", func(t *testing.T) {
		clause, args := Filter{Denied: true}.where()
		if clause != "0" {
			t.Errorf("clause = %q, want 0 — a bug that lets a denied read through must return nothing, not everything", clause)
		}
		if len(args) != 0 {
			t.Errorf("args = %v", args)
		}
	})

	t.Run("moderator is unrestricted", func(t *testing.T) {
		clause, _ := Filter{Unrestricted: true}.where()
		if clause != "1" {
			t.Errorf("clause = %q, want 1", clause)
		}
	})

	t.Run("member sees own rows unconditionally", func(t *testing.T) {
		clause, args := Filter{PersonID: "p-1", Group: "spejder"}.where()
		if !strings.Contains(clause, "g.authorPersonId = ?") {
			t.Errorf("clause does not include own rows: %s", clause)
		}
		if !strings.Contains(clause, "g.hiddenAt IS NULL") {
			t.Errorf("clause does not exclude hidden rows: %s", clause)
		}
		if len(args) != 5 {
			t.Errorf("args = %v, want person, nathejk, public, and the group pair", args)
		}
	})

	t.Run("no group means no group-scoped rows", func(t *testing.T) {
		clause, _ := Filter{PersonID: "p-1"}.where()
		if strings.Contains(clause, "g.authorGroup") {
			t.Errorf("a group-less viewer got a group clause: %s", clause)
		}
	})
}

func TestPublicFeedTakesNoCaller(t *testing.T) {
	// The public page shares a host with the app, so a member's browser sends its session
	// cookie to it. This read must be incapable of noticing (PRD 019 §8): the signature has
	// no caller, and the query pins audience and hidden state itself.
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta("g.audience = ?")).
		WithArgs("2026", AudiencePublic, 20, 0).
		WillReturnRows(glimtRows())

	if _, err := (querier{db: db}).PublicFeed("2026", 20, 0); err != nil {
		t.Fatalf("PublicFeed: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}

func TestModerationQueuePutsUnreviewedReportsFirst(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta("(g.reportCount > 0 AND g.hiddenAt IS NULL) DESC")).
		WillReturnRows(glimtRows())
	if _, err := (querier{db: db}).Moderation("2026", 20, 0); err != nil {
		t.Fatalf("Moderation: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}

func TestExpiredReturnsThumbnailRefsToo(t *testing.T) {
	// Forgetting the thumbnails would leave recognisable images on disk while technically
	// having "purged the glimt" — the mistake the portrait purge test exists to catch.
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta("FROM glimt g")).
		WillReturnRows(sqlmock.NewRows([]string{"glimtId", "full", "thumbs"}).
			AddRow("g-1", refA+","+refC, refB))

	got, err := querier{db: db}.Expired("2026", time.Now(), 100)
	if err != nil {
		t.Fatalf("Expired: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 expired glimt, got %d", len(got))
	}
	if len(got[0].Refs) != 3 {
		t.Errorf("refs = %v, want both full refs and the thumbnail", got[0].Refs)
	}
}

func TestVersionChangesWhenSomethingIsHidden(t *testing.T) {
	// A version that only tracked creations would leave a reported glimt in every client
	// that already held it.
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	created := time.Date(2026, 9, 17, 21, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta("COUNT(*)")).
		WillReturnRows(sqlmock.NewRows([]string{"c", "newest", "hidden"}).
			AddRow(3, created, nil))
	before, err := querier{db: db}.Version("2026", Filter{PersonID: "p-1"})
	if err != nil {
		t.Fatalf("Version: %v", err)
	}

	mock.ExpectQuery(regexp.QuoteMeta("COUNT(*)")).
		WillReturnRows(sqlmock.NewRows([]string{"c", "newest", "hidden"}).
			AddRow(3, created, created.Add(time.Minute)))
	after, err := querier{db: db}.Version("2026", Filter{PersonID: "p-1"})
	if err != nil {
		t.Fatalf("Version: %v", err)
	}

	if before == after {
		t.Errorf("version unchanged after a takedown: %q", before)
	}
}

func TestByHoldRefusesAnEmptyNumber(t *testing.T) {
	// An empty number matches the column default, which every crew glimt carries — it would
	// return them all as though they were one hold's collection.
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	got, err := querier{db: db}.ByHold("2026", "", Filter{PersonID: "p-1"}, 20, 0)
	if err != nil || got != nil {
		t.Errorf("ByHold(\"\") = %v, %v; want nil, nil", got, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unexpected query: %v", err)
	}
}

func TestGetIsUnfilteredByDesign(t *testing.T) {
	// Get backs DELETE authorization and the media handler, both of which need the row in
	// order to decide. It returns hidden rows too — the caller checks.
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	hidden := time.Date(2026, 9, 17, 23, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta("FROM glimt g")).
		WithArgs("2026", "g-1").
		WillReturnRows(glimtRows().
			AddRow("g-1", "2026", "p-1", "spejder", "42", "Ørnene", "group", "", hidden, 0, hidden, "p-team", 2))
	mock.ExpectQuery(regexp.QuoteMeta("FROM glimt_media")).
		WillReturnRows(sqlmock.NewRows([]string{
			"glimtId", "ordinal", "blobRef", "thumbRef", "kind", "contentType", "bytes", "width", "height", "durationMs",
		}))

	got, found, err := querier{db: db}.Get("2026", "g-1")
	if err != nil || !found {
		t.Fatalf("Get = %v, %v, %v", got, found, err)
	}
	if got.HiddenAt == nil {
		t.Error("Get filtered out a hidden row; the caller must be able to see it to decide")
	}
	if got.ReportCount != 2 || got.HiddenBy != "p-team" {
		t.Errorf("moderation fields lost: %+v", got)
	}
}

func TestClampLimit(t *testing.T) {
	// An unbounded query on the post-race browse is the difference between a slow page and
	// a service that stops answering.
	if got := clampLimit(0); got != 20 {
		t.Errorf("clampLimit(0) = %d, want the default", got)
	}
	if got := clampLimit(-5); got != 20 {
		t.Errorf("clampLimit(-5) = %d, want the default", got)
	}
	if got := clampLimit(10_000); got != 200 {
		t.Errorf("clampLimit(10000) = %d, want the cap", got)
	}
	if got := clampLimit(50); got != 50 {
		t.Errorf("clampLimit(50) = %d", got)
	}
	if got := clampOffset(-1); got != 0 {
		t.Errorf("clampOffset(-1) = %d", got)
	}
}

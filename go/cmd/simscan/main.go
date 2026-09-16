// Command simscan injects a single, complete "patrol arrived at a post" story into the event stream, so
// PRD 016's attribution, verdict and reveal paths can be exercised on a device against real entities.
//
// # Why this is a separate command and not a `seed` case
//
// `cmd/seed` **refuses real event years** on purpose, and that guard is correct: it publishes a catalogue of
// synthetic members, and doing so into a live year would put junk into every consumer's projection. Adding a
// bypass flag there would weaken the guard for every case it protects.
//
// This command needs the opposite of that guard, because the things it references — a checkpoint, a patrol —
// only exist in the *real* year. So it lives behind its own obviously-named door, and takes the risk
// seriously in two ways instead of one:
//
//  1. **It is a dry run unless you pass -confirm.** Running it prints exactly what it would publish and
//     exits. Nothing about the default invocation touches the broker.
//  2. **It says what cannot be undone.** The stream is append-only and every projection rebuilds from
//     sequence zero on boot, so a published scan is permanent until someone purges the stream — and the
//     broker is shared with the other local services (hq, tilmelding, skan), which will show it too.
//
// # What it publishes
//
// Three facts, because a scan on its own does not attribute to anything:
//
//  1. a crew member, assigned to the `postmandskab` section (this is what makes them post personnel);
//  2. a `checkpersonnel` shift putting that person on the named checkpoint, spanning now;
//  3. a `qr.scanned` for the named team, scanned by that person.
//
// The attribution join is `scanner was on a registered shift at that post at that moment`, so all three are
// required — and the shift's window is what makes the scan land on the post rather than nowhere.
//
// # The scan's timestamp is the publish time, not a field
//
// `scan`'s projector takes `uts` from `msg.Time()`, not from the body (there is no time in `qr.scanned`).
// So the scan happens *now* and cannot be backdated from here. That is why the shift is opened around the
// current moment rather than around the event's real hours, and it is worth knowing before reading the
// on-time verdict: if the post's window is the real event's, a scan today is legitimately "for tidligt".
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/jrgensen/cqrs"
	"github.com/jrgensen/stream/jetstream"
	"github.com/jrgensen/stream/metatagger"
	"github.com/nathejk/shared-go/messages"
	"github.com/nathejk/shared-go/types"
)

type event struct {
	subject string
	what    string
	body    any
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "simscan: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	dsn := flag.String("jetstream-dsn", os.Getenv("JETSTREAM_DSN"), "NATS JetStream DSN")
	year := flag.String("year", os.Getenv("EVENT_YEAR"), "Event year to publish under")

	checkpointID := flag.String("checkpoint", "", "Checkpoint id the patrol arrives at (required)")
	teamID := flag.String("team", "", "Team id of the arriving patrol (required)")
	teamNumber := flag.String("team-number", "", "Patrol number, as printed (required)")

	// Deterministic defaults, so running twice does not accumulate a pile of crew members: the
	// crewmember projection upserts on id, and the scan is keyed (qrId, uts).
	scannerID := flag.String("scanner-id", "test-postmand-sim", "Crew member id to create and put on the post")
	scannerName := flag.String("scanner-name", "TEST Postmand Sim", "Crew member name — keep it legibly fake")
	scannerPhone := flag.String("scanner-phone", "+4599000911", "Crew member phone (+4599 marks it synthetic)")

	shiftBefore := flag.Duration("shift-before", time.Hour, "How far before now the shift opens")
	shiftAfter := flag.Duration("shift-after", 12*time.Hour, "How far after now the shift closes")

	// Off by default, because it rewrites a *real* post's advertised hours — see the warning in dryRun.
	fixWindow := flag.Bool("fix-window", false,
		"Also republish the checkpoint's open window around now, so the scan reads as \"p\u00e5 tid\"")
	windowBefore := flag.Duration("window-before", 30*time.Minute, "How far before now the post's window opens")
	windowAfter := flag.Duration("window-after", 2*time.Hour, "How far after now the post's window closes")

	lat := flag.String("lat", "", "Optional scan latitude")
	lng := flag.String("lng", "", "Optional scan longitude")

	confirm := flag.Bool("confirm", false, "Actually publish. Without this it is a dry run.")
	flag.Parse()

	if *year == "" {
		return errors.New("no -year and no EVENT_YEAR set")
	}
	if *checkpointID == "" || *teamID == "" || *teamNumber == "" {
		return errors.New("-checkpoint, -team and -team-number are all required")
	}

	now := time.Now()
	shift := types.TimeRange{Start: now.Add(-*shiftBefore), End: now.Add(*shiftAfter)}

	// nil unless asked for, so the checkpoint patch is absent rather than empty.
	var window *types.TimeRange
	if *fixWindow {
		window = &types.TimeRange{Start: now.Add(-*windowBefore), End: now.Add(*windowAfter)}
	}

	events := storyFor(*year, *checkpointID, *teamID, *teamNumber,
		*scannerID, *scannerName, *scannerPhone, shift, window, *lat, *lng)

	if !*confirm {
		return dryRun(*year, events, shift, window)
	}
	return publish(*dsn, *year, events)
}

// storyFor builds the three-part story. Split out from publishing so the dry run shows precisely the bodies
// that would go on the wire, rather than a description of them.
func storyFor(
	year, checkpointID, teamID, teamNumber string,
	scannerID, scannerName, scannerPhone string,
	shift types.TimeRange,
	window *types.TimeRange,
	lat, lng string,
) []event {
	// A stable shift id derived from the pair it describes: re-running updates the same shift (the
	// projector upserts on it) instead of stacking overlapping ones, which would attribute the same scan
	// several times over.
	shiftID := fmt.Sprintf("test-shift-%s-%s", scannerID, checkpointID)

	scan := messages.NathejkQrScanned{
		// Legibly fake, and it is only an identifier here: the projector takes the team from the body,
		// not by resolving the code.
		QrID:         types.QrID("TEST-SIM-" + teamNumber),
		TeamID:       types.TeamID(teamID),
		TeamNumber:   teamNumber,
		ScannerID:    scannerID,
		ScannerPhone: types.PhoneNumber(scannerPhone),
		Remark:       "Simuleret ankomst (simscan)",
	}
	scan.Location.Latitude = lat
	scan.Location.Longitude = lng

	out := []event{
		{
			subject: fmt.Sprintf("NATHEJK.%s.crewmember.%s.updated", year, scannerID),
			what:    "crew member exists",
			body: messages.NathejkCrewMemberUpdated{
				UserID: types.UserID(scannerID),
				Name:   scannerName,
				Phone:  types.PhoneNumber(scannerPhone),
				Email:  types.EmailAddress(scannerID + "@example.invalid"),
			},
		},
		{
			// This is what makes them *postmandskab*: the app role is classified from the section slug.
			subject: fmt.Sprintf("NATHEJK.%s.crewmember.%s.section.assigned", year, scannerID),
			what:    "…in the postmandskab section (this is what sets the app role)",
			body: messages.NathejkCrewMemberSectionAssigned{
				UserID:      types.UserID(scannerID),
				SectionSlug: types.Slug("postmandskab"),
			},
		},
		{
			// Republished so the label survives a projection rebuild even if nothing else supplies it.
			// Idempotent: the projector upserts on slug.
			subject: fmt.Sprintf("NATHEJK.%s.section.postmandskab.added", year),
			what:    "…and the section has a label",
			body:    messages.NathejkSectionAdded{Slug: "postmandskab", Label: "Postmandskab"},
		},
		{
			subject: fmt.Sprintf("NATHEJK.%s.checkpersonnel.%s.added", year, shiftID),
			what:    "on shift at the checkpoint, spanning now — this is what attributes the scan",
			body: messages.NathejkCheckpersonnelAdded{
				UserID:       types.UserID(scannerID),
				CheckpointID: types.CheckpointID(checkpointID),
				TimeRange:    &shift,
			},
		},
		{
			subject: fmt.Sprintf("NATHEJK.%s.qr.%s.scanned", year, scan.QrID),
			what:    "the patrol's code scanned by that person (uts = publish time)",
			body:    scan,
		},
	}

	// The window goes first when asked for, so the post's hours are already right by the time the scan
	// lands. Ordering is cosmetic — the verdict is computed on read, joining the scan to whatever the
	// checkpoint's window says at query time — but reading the output in causal order is worth something.
	//
	// A **patch**: only FixedTimeRange is set, so the post's name, address and position are left alone
	// (nil means unchanged, and the projector only writes the columns present). Setting more here would
	// overwrite real organizer data with defaults.
	if window != nil {
		out = append([]event{{
			subject: fmt.Sprintf("NATHEJK.%s.checkpoint.%s.updated", year, checkpointID),
			what:    "REWRITES THE REAL POST'S OPEN HOURS to span now, so the verdict reads \"på tid\"",
			body: messages.NathejkCheckpointUpdated{
				CheckpointID:   types.CheckpointID(checkpointID),
				FixedTimeRange: window,
			},
		}}, out...)
	}
	return out
}

// dryRun prints what would be published, and what publishing cannot undo.
func dryRun(year string, events []event, shift types.TimeRange, window *types.TimeRange) error {
	fmt.Printf("DRY RUN — nothing published. Add -confirm to publish.\n\n")
	fmt.Printf("year   %s\n", year)
	fmt.Printf("shift  %s → %s\n", shift.Start.Format(time.RFC3339), shift.End.Format(time.RFC3339))
	if window != nil {
		fmt.Printf("window %s → %s (rewritten)\n", window.Start.Format(time.RFC3339), window.End.Format(time.RFC3339))
	}
	fmt.Printf("scan   %s (the projector timestamps it at publish time)\n\n", time.Now().Format(time.RFC3339))

	for i, e := range events {
		body, err := json.MarshalIndent(e.body, "    ", "  ")
		if err != nil {
			return fmt.Errorf("marshal %s: %w", e.subject, err)
		}
		fmt.Printf("%d. %s\n   %s\n    %s\n\n", i+1, e.subject, e.what, body)
	}

	fmt.Printf("This cannot be undone. The stream is append-only and every projection rebuilds from\n")
	fmt.Printf("sequence zero on boot, so these events persist until the stream is purged — and the\n")
	fmt.Printf("broker is shared with the other local services, which will project them too.\n")
	if window != nil {
		fmt.Printf("\nAnd note the window rewrite is not scoped to one patrol: it changes this post's\n")
		fmt.Printf("advertised hours for EVERY patrol's verdict, and hq will show the new hours.\n")
	}
	return nil
}

func publish(dsn, year string, events []event) error {
	if dsn == "" {
		return errors.New("no JETSTREAM_DSN set and no -jetstream-dsn given")
	}

	js, err := jetstream.New(dsn)
	if err != nil {
		return fmt.Errorf("connect jetstream: %w", err)
	}
	defer js.Close()

	// Tagged as this tool, so a synthetic arrival is identifiable as synthetic from its metadata alone —
	// the one thing that makes it distinguishable later, since it references real entities.
	publisher, err := metatagger.New(js, messages.Metadata{Producer: "hej-simscan", Version: "synthetic"})
	if err != nil {
		return fmt.Errorf("create publisher: %w", err)
	}

	for _, e := range events {
		msg := publisher.MessageFunc()(cqrs.SubjectFromStr(e.subject))
		if err := msg.SetBody(e.body); err != nil {
			return fmt.Errorf("set body %s: %w", e.subject, err)
		}
		if err := publisher.Publish(msg); err != nil {
			return fmt.Errorf("publish %s: %w", e.subject, err)
		}
		fmt.Printf("  published %s\n", e.subject)
	}

	fmt.Printf("\n%d events published under year %s.\n", len(events), year)
	fmt.Printf("The api projects on boot; restart it (or wait for the live consumer) to see them.\n")
	return nil
}

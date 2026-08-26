// Command seed publishes synthetic member events to the dev broker so that edge
// cases which do not occur naturally can be exercised on demand.
//
// # What this is and is not
//
// It is NOT how you get usable local data. The shared dev broker already carries a
// realistic dataset (~800 live participants for 2026), and the projection rebuilds from
// it on every boot — that is the answer to "I need data", and it needs no tooling. See
// the repository README.
//
// This exists for the cases that dataset does *not* contain, or contains only by luck.
// Task 078's replay found the sharpest example: three of the seven app roles
// (postmandskab, guide, samarit) have **no members at all**, because no real crew member
// is assigned to a capability section. So the samarit SOS page and the whole of PRD 007's
// access matrix cannot be exercised against real data. Waiting for an organizer to assign
// someone is not a development strategy.
//
// # Why it publishes events instead of inserting rows
//
// Nothing in this architecture writes to the database directly (PRD 008 §8): SQL is
// projection-only and every state change is an event. A seeder that INSERTed rows would
// test a table shape rather than the projector, and would drift from the real message
// bodies the moment anything upstream changed. Publishing real subjects means seeded cases
// travel the same code path as production data — including the parts most likely to be
// wrong, like phone normalization and out-of-order convergence.
//
// # Why a separate, obviously fake year
//
// It publishes under -year, defaulting to 9999, and refuses to touch a real event year.
// Two reasons. The broker is a shared, org-level service, so anything published here is
// visible to every other consumer (hq, tilmelding) and cannot be un-published; confining
// it to a sentinel year keeps it out of every real projection. And `hej` reads exactly one
// year (EVENT_YEAR, task 077), so pointing a dev instance at 9999 shows the seeded cases
// and nothing else.
//
// Names are deliberately, unmistakably fake — "TEST Delt Nummer", not "Freja Hansen".
// Task 065 recorded the reason from experience in both directions: realistic-looking fake
// data gets treated as real (I once reported a privacy incident over a constructed
// dataset), and fake-looking real data gets treated casually. Legible fakeness is the
// cheap mitigation, and a sentinel year is not legible enough when you are looking at a
// single row.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/jrgensen/cqrs"
	"github.com/jrgensen/stream/jetstream"
	"github.com/jrgensen/stream/metatagger"
	"github.com/nathejk/shared-go/messages"
	"github.com/nathejk/shared-go/types"
)

// realYears are refused outright.
//
// A guard rather than a warning: the failure mode is publishing junk into a live event's
// projection, on a broker shared with other services, where it cannot be withdrawn. A
// mistyped -year is an ordinary mistake and this makes it a non-event.
var realYears = map[string]bool{
	"2023": true, "2024": true, "2025": true, "2026": true, "2027": true,
	"2028": true, "2029": true, "2030": true,
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "seed: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	dsn := flag.String("jetstream-dsn", os.Getenv("JETSTREAM_DSN"), "NATS JetStream DSN")
	year := flag.String("year", "9999", "Sentinel event year to publish under (never a real one)")
	list := flag.Bool("list", false, "List the available cases and exit")
	only := flag.String("case", "", "Seed a single case by name (default: all)")
	flag.Parse()

	cases := allCases()

	if *list {
		for _, c := range cases {
			fmt.Printf("  %-24s %s\n", c.name, c.what)
		}
		return nil
	}

	if realYears[*year] {
		return fmt.Errorf("refusing to publish into real event year %q: this tool writes to "+
			"the SHARED broker and cannot un-publish. Use the default 9999", *year)
	}
	if *dsn == "" {
		return errors.New("no JETSTREAM_DSN set and no -jetstream-dsn given")
	}

	selected := cases
	if *only != "" {
		selected = nil
		for _, c := range cases {
			if c.name == *only {
				selected = append(selected, c)
			}
		}
		if selected == nil {
			return fmt.Errorf("no such case %q (try -list)", *only)
		}
	}

	js, err := jetstream.New(*dsn)
	if err != nil {
		return fmt.Errorf("connect jetstream: %w", err)
	}
	defer js.Close()

	// Tagged as this tool, not as the api: a synthetic event should be identifiable as
	// synthetic from its metadata alone, without knowing the year convention.
	publisher, err := metatagger.New(js, messages.Metadata{Producer: "hej-seed", Version: "synthetic"})
	if err != nil {
		return fmt.Errorf("create publisher: %w", err)
	}

	published := 0
	for _, c := range selected {
		for _, e := range c.events(*year) {
			msg := publisher.MessageFunc()(cqrs.SubjectFromStr(e.subject))
			if err := msg.SetBody(e.body); err != nil {
				return fmt.Errorf("%s: set body: %w", c.name, err)
			}
			if err := publisher.Publish(msg); err != nil {
				return fmt.Errorf("%s: publish %s: %w", c.name, e.subject, err)
			}
			published++
		}
		fmt.Printf("  seeded %-24s %s\n", c.name, c.what)
	}

	fmt.Printf("\n%d events published under year %s.\n", published, *year)
	fmt.Printf("Set EVENT_YEAR=%s on the api and restart it to read them.\n", *year)
	return nil
}

type event struct {
	subject string
	body    any
}

type seedCase struct {
	name string
	what string
	// events takes the year so no case can hardcode one.
	events func(year string) []event
}

// scout builds a spejder.updated event.
func scout(year, id, name, phone, guardian, birthday string) event {
	return event{
		subject: fmt.Sprintf("NATHEJK.%s.spejder.%s.updated", year, id),
		body: messages.NathejkScoutUpdated{
			MemberID:     types.MemberID(id),
			Name:         name,
			Phone:        types.PhoneNumber(phone),
			PhoneContact: types.PhoneNumber(guardian),
			BirthDate:    types.Date(birthday),
			Address:      "Testvej 1",
			PostalCode:   "9999",
			City:         "Testby",
			Email:        types.EmailAddress(strings.ToLower(id) + "@example.invalid"),
		},
	}
}

// crew builds the three events a classified crew member needs: the person, the section
// assignment, and the section's label.
func crew(year, id, name, phone, sectionSlug, sectionLabel string) []event {
	out := []event{
		{
			subject: fmt.Sprintf("NATHEJK.%s.crewmember.%s.updated", year, id),
			body: messages.NathejkCrewMemberUpdated{
				UserID: types.UserID(id),
				Name:   name,
				Phone:  types.PhoneNumber(phone),
				Email:  types.EmailAddress(strings.ToLower(id) + "@example.invalid"),
			},
		},
		{
			subject: fmt.Sprintf("NATHEJK.%s.crewmember.%s.section.assigned", year, id),
			body: messages.NathejkCrewMemberSectionAssigned{
				UserID:      types.UserID(id),
				SectionSlug: types.Slug(sectionSlug),
			},
		},
	}
	if sectionLabel != "" {
		out = append(out, event{
			subject: fmt.Sprintf("NATHEJK.%s.section.%s.added", year, sectionSlug),
			body:    messages.NathejkSectionAdded{Slug: types.Slug(sectionSlug), Label: sectionLabel},
		})
	}
	return out
}

// allCases is the catalogue. Each entry exists because something in the app branches on
// it and the real dataset either cannot produce it, or produces it only by luck.
func allCases() []seedCase {
	cases := []seedCase{
		{
			// The reason this tool earns its keep. Task 078: nobody in the real data
			// has a capability role, so these pages have never been opened by an
			// account entitled to them.
			name: "capability-crew",
			what: "one crew member per capability role (samarit/postmandskab/guide) — none exist in real data",
			events: func(y string) []event {
				var out []event
				out = append(out, crew(y, "test-samarit", "TEST Samarit", "+4599000010", "samarit", "Samarit")...)
				out = append(out, crew(y, "test-postmand", "TEST Postmand", "+4599000011", "postmandskab", "Postmandskab")...)
				out = append(out, crew(y, "test-guide", "TEST Guide", "+4599000012", "guides", "Guides")...)
				return out
			},
		},
		{
			name: "shared-phone",
			what: "two different people on +4599000020 — the chooser's genuine case",
			events: func(y string) []event {
				return []event{
					scout(y, "test-share-a", "TEST Delt Nummer A", "+4599000020", "+4599000021", "2010-06-03"),
					scout(y, "test-share-b", "TEST Delt Nummer B", "+4599000020", "+4599000021", "2011-07-04"),
				}
			},
		},
		{
			// Distinct from shared-phone, and the distinction is the point: here the
			// chooser cannot help, because both options read identically. 70 of 85 real
			// shared numbers look like this (PRD 006 §11 Q9).
			name: "duplicate-registration",
			what: "the same person twice on +4599000030 — the case the chooser CANNOT resolve",
			events: func(y string) []event {
				return []event{
					scout(y, "test-dup-1", "TEST Dobbelt Tilmeldt", "+4599000030", "+4599000031", "2010-01-02"),
					scout(y, "test-dup-2", "TEST Dobbelt Tilmeldt", "+4599000030", "+4599000031", "2010-01-02"),
				}
			},
		},
		{
			name: "unmapped-section",
			what: "crew in a slug the classifier does not know — exercises the warning and the least-privileged fallback",
			events: func(y string) []event {
				return crew(y, "test-unmapped", "TEST Ukendt Sektion", "+4599000040",
					"en-sektion-ingen-har-klassificeret", "En Sektion Ingen Har Klassificeret")
			},
		},
		{
			name: "unusable-phones",
			what: "guardian numbers that cannot be normalized (7 digits; two numbers in one field)",
			events: func(y string) []event {
				return []event{
					scout(y, "test-badphone-1", "TEST Kort Foraeldrenummer", "+4599000050", "3068640", "2010-03-04"),
					scout(y, "test-badphone-2", "TEST To Foraeldrenumre", "+4599000051",
						"Mor: 24281097 eller Far: 22239313", "2010-03-05"),
				}
			},
		},
		{
			name: "no-guardian",
			what: "spejder with no guardian number — PRD 005 has nothing to verify",
			events: func(y string) []event {
				return []event{scout(y, "test-noguardian", "TEST Uden Foraelder", "+4599000060", "", "2010-05-06")}
			},
		},
		{
			name: "no-birthday",
			what: "spejder with no birthdate — the column is nullable and the parser must not dead-letter",
			events: func(y string) []event {
				return []event{scout(y, "test-nobirthday", "TEST Uden Foedselsdag", "+4599000070", "+4599000071", "")}
			},
		},
		{
			name: "no-own-phone",
			what: "spejder with only a guardian number — must NOT be able to log in (PRD 006 §11 Q13)",
			events: func(y string) []event {
				return []event{scout(y, "test-noownphone", "TEST Uden Egen Telefon", "", "+4599000080", "2010-08-09")}
			},
		},
		{
			name: "deleted",
			what: "a member who is then deleted — their login must stop working",
			events: func(y string) []event {
				return []event{
					scout(y, "test-deleted", "TEST Slettet", "+4599000090", "+4599000091", "2010-09-10"),
					{
						subject: fmt.Sprintf("NATHEJK.%s.spejder.test-deleted.deleted", y),
						body:    messages.NathejkMemberAdded{MemberID: "test-deleted"},
					},
				}
			},
		},
		{
			// An ordering the real stream cannot be asked to produce, and the one task
			// 074 nearly got wrong: the assignment lands before the person exists and
			// before the section is named.
			name: "out-of-order-crew",
			what: "section assignment published BEFORE the crew member and the section label",
			events: func(y string) []event {
				const id = "test-outoforder"
				return []event{
					{
						subject: fmt.Sprintf("NATHEJK.%s.crewmember.%s.section.assigned", y, id),
						body: messages.NathejkCrewMemberSectionAssigned{
							UserID: types.UserID(id), SectionSlug: "samarit",
						},
					},
					{
						subject: fmt.Sprintf("NATHEJK.%s.crewmember.%s.updated", y, id),
						body: messages.NathejkCrewMemberUpdated{
							UserID: types.UserID(id), Name: "TEST Ude Af Raekkefoelge",
							Phone: "+4599000100",
						},
					},
					{
						subject: fmt.Sprintf("NATHEJK.%s.section.samarit.added", y),
						body:    messages.NathejkSectionAdded{Slug: "samarit", Label: "Samarit"},
					},
				}
			},
		},
		{
			name: "goegler-signup-only",
			what: "gøgler with a signup and no update — a third of real gøglere look like this",
			events: func(y string) []event {
				return []event{{
					subject: fmt.Sprintf("NATHEJK.%s.gøgler.test-goegler.signedup", y),
					// A map, not a struct: the gøgler shapes live in the person package
					// (there are none in shared-go) and importing a projection's types
					// into a publisher would be backwards.
					body: map[string]any{
						"teamId": "test-goegler",
						"name":   "TEST Goegler",
						"phone":  "+4599000110",
						"email":  "test-goegler@example.invalid",
					},
				}}
			},
		},
		{
			name: "bandit",
			what: "a bandit (senior in a klan) with an arm number — no real arm numbers exist yet",
			events: func(y string) []event {
				const id = "test-bandit"
				return []event{
					{
						subject: fmt.Sprintf("NATHEJK.%s.senior.%s.updated", y, id),
						body: messages.NathejkSeniorUpdated{
							MemberID: types.MemberID(id), Name: "TEST Bandit",
							Phone: "+4599000120", BirthDate: "1995-04-05",
						},
					},
					{
						subject: fmt.Sprintf("NATHEJK.%s.bandit.%s.armNumber.assigned", y, id),
						body:    map[string]any{"memberId": id, "armNumber": "999"},
					},
				}
			},
		},
	}

	sort.Slice(cases, func(i, j int) bool { return cases[i].name < cases[j].name })
	return cases
}

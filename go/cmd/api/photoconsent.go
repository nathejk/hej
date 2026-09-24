package main

import (
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/jrgensen/cqrs"

	"nathejk.dk/nathejk/table/album"
	"nathejk.dk/nathejk/table/patrolphoto"
)

// A photo refusal takes the patrol's photographs out of every album (task 397).
//
// # Why this is needed on top of the diploma's gate
//
// `patrolphoto.Cover` stops returning a refused patrol's photograph, which covers the diploma. But a photograph
// already copied into the library — by the diploma-album button or by a curator — is its own record, and an album
// holding it would keep showing it. So when hq records a refusal, every live album membership of a photograph
// tagged with that patrol is removed. The photograph stays in the library: a refusal can be withdrawn, and
// deleting the bytes is a curator's decision, not this one's.
//
// # Live messages only
//
// This is a reaction, not a projection: it publishes. Projections replay the stream from zero on every boot, and
// replaying an old refusal would take down photographs a curator put back after the patrol withdrew it. So a
// message older than this process is ignored. A refusal that arrived while the app was down is caught by the
// diploma-album button, which sweeps every refused patrol before filing anything.

// consentReactor removes a refusing patrol's photographs from albums as the refusal arrives.
type consentReactor struct {
	// app is set once the application exists; the broker may connect before it does.
	app     atomic.Pointer[application]
	started time.Time
	logger  *slog.Logger
}

func newConsentReactor(logger *slog.Logger) *consentReactor {
	return &consentReactor{started: time.Now(), logger: logger}
}

func (c *consentReactor) Consumes() []cqrs.Subject {
	return []cqrs.Subject{cqrs.SubjectFromStr("NATHEJK:*.patrulje.*.photoconsented")}
}

func (c *consentReactor) HandleMessage(msg cqrs.Message) error {
	if msg.Time().Before(c.started) {
		return nil
	}
	var body patrolphoto.PhotoConsentSet
	if err := msg.Body(&body); err != nil {
		return fmt.Errorf("photoconsent reaction: %w", err)
	}
	if !body.Refused() {
		// Consent restored. Nothing is put back automatically: which albums the photographs belong in is the
		// curator's call, and a removal stays a removal until somebody re-adds the photograph by hand.
		return nil
	}
	parts := msg.Subject().Parts()
	if len(parts) < 4 {
		return fmt.Errorf("photoconsent reaction: unexpected subject %q", msg.Subject().Subject())
	}
	year, teamID := parts[1], string(body.TeamID)
	if teamID == "" {
		teamID = parts[3]
	}
	app := c.app.Load()
	if app == nil {
		c.logger.Error("a photo refusal arrived before the application was ready; press the diploma-album button to apply it",
			"year", year, "team", teamID)
		return nil
	}
	n, err := app.removeRefusedFromAlbums(year, teamID)
	if err != nil {
		return fmt.Errorf("photoconsent reaction: %w", err)
	}
	c.logger.Info("a photo refusal removed the patrol's photographs from albums",
		"year", year, "team", teamID, "removed", n)
	return nil
}

var _ cqrs.Consumer = (*consentReactor)(nil)

// removeRefusedFromAlbums takes every photograph tagged with a patrol out of every album that shows it.
//
// Keyed by photograph, as every removal is since task 386, with the ordinal alongside for the record. Idempotent:
// only live memberships are listed, so a second run publishes nothing.
func (app *application) removeRefusedFromAlbums(year, teamID string) (int, error) {
	if app.models.PhotoCurator == nil {
		return 0, fmt.Errorf("the photo library is unavailable")
	}
	items, err := app.models.PhotoCurator.TeamAlbumItems(year, teamID)
	if err != nil {
		return 0, err
	}
	removed := 0
	for _, it := range items {
		if err := app.publishAlbum(year, album.VerbItemRemoved, it.AlbumID, album.ItemRemoved{
			AlbumID:   it.AlbumID,
			Year:      year,
			PhotoID:   it.PhotoID,
			Ordinal:   it.Ordinal,
			Reason:    "Fototilladelse: patruljen har frabedt sig billeder",
			RemovedAt: time.Now().UTC(),
		}); err != nil {
			return removed, err
		}
		removed++
	}
	return removed, nil
}

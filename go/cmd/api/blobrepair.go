package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"nathejk.dk/internal/blob"
	"nathejk.dk/internal/imaging"
)

// Rebuilding a derived rendition that has gone missing (task 430).
//
// # Why this can exist at all
//
// Task 429 split the blob store into `original/` (backed up) and `cache/` (not). That split is only
// honest if the cache can be refilled, and the thing that makes refilling possible is the rule in the
// `blob` package comment: **a derived ref is a name, not a checksum.** A rebuilt thumbnail is written
// back under the ref the projection already holds, via `blob.Store.PutAs`, so no row changes and
// nothing has to be published from a read path — which is what made every other approach expensive
// (see task 430's superseded analysis).
//
// # The store cannot do this itself
//
// A `blob.Store` is handed opaque bytes and a ref. It has no idea that one object is a 320px resize of
// another, and teaching it would mean teaching it about images. But **every serving call site already
// knows**: it holds the source ref, the target ref and the variant that was asked for. So the recipe
// travels from the call site as a `renditionRepair`, and the store stays a store.
//
// # Not on the request's critical path
//
// A miss serves the **source** image immediately and rebuilds in the background. A cold cache under
// the post-race load PRD 019 §0a.3 describes is exactly when nobody wants a resize between the request
// and the response, and the fallback costs nothing to implement because every one of these surfaces
// already degrades to the full image when a rendition is absent — that fallback is what made excluding
// `cache/` from the backup safe in the first place.
//
// The one thing that fallback must not do is let a client cache the full-size image *as* the thumbnail
// for a year. See `degradedRenditionCacheControl`.

// renditionRebuildTimeout bounds one rebuild.
//
// Generous: it is a decode, a resize and an encode of one image, off the request path, and a rebuild
// killed halfway is a rebuild that has to happen again on the next request.
const renditionRebuildTimeout = 30 * time.Second

// degradedRenditionCacheControl is what the **fallback** answer carries, and it is the reason the
// fallback is safe.
//
// The normal answers on these routes are `immutable` with a year's `max-age`, which is correct when the
// bytes really are the rendition that was asked for. On the degraded path they are not: they are the
// full-size source standing in for a thumbnail. Serving *those* bytes `immutable` would mean a client
// unlucky enough to arrive during the rebuild window caches a multi-megabyte image under the thumbnail's
// URL and never revalidates — for a year, on a phone, on a page that shows sixty of them.
//
// So: a short window and explicitly **not** `immutable`, so the next request picks up the real
// rendition. `private` because the degraded bytes may be a member's photograph and this constant is
// shared by the authenticated and public routes alike; the public route's own answer is `public`, but
// erring toward `private` on a transient path costs one shared-cache miss and cannot leak anything.
const degradedRenditionCacheControl = "private, max-age=30"

// renditionRepair says how to rebuild one derived rendition, or — as its zero value — that the ref being
// served is not a rebuildable rendition at all.
//
// The zero value being "no repair" is deliberate: a call site that resolves to a full image, or to a
// portrait with nothing to rebuild from, says so by passing nothing, and the serving path then behaves
// exactly as it did before this file existed.
type renditionRepair struct {
	// Target is the recorded ref of the rendition: the name to write the rebuilt bytes back under,
	// **not** the hash of those bytes.
	Target blob.Ref

	// Source is the object to rebuild from. Always something the store holds as an original.
	Source blob.Ref

	// Edge is the longest edge of the rendition, in pixels.
	Edge int

	// Quality is the JPEG quality to encode at.
	Quality int
}

// possible reports whether this plan can be acted on.
//
// `Target != Source` matters more than it looks: when a variant resolves to the full image — the
// existing fallback for a portrait with no such rendition, or an item with no thumbnail — the two refs
// are the same object, and "rebuilding" it would mean writing a resize of a photograph over the
// photograph. `blob.PutAs` refuses that outright as its last line of defence, but it should never be
// asked.
func (p renditionRepair) possible() bool {
	return p.Target != "" && p.Source != "" && p.Target != p.Source && p.Edge > 0 && p.Quality > 0
}

// repairRenditionInBackground starts a rebuild and returns immediately.
//
// The context is deliberately **detached** from the request's. The request is about to be answered from
// the source image, so the client is gone within milliseconds; a rebuild on `r.Context()` would be
// cancelled the moment the response finished and the cache would stay empty forever, one abandoned
// resize per request.
func (app *application) repairRenditionInBackground(r *http.Request, plan renditionRepair) {
	if !plan.possible() {
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), renditionRebuildTimeout)
	go func() {
		defer cancel()
		if err := app.repairRendition(ctx, plan); err != nil {
			// Warn, not error: the request it came from was answered correctly from the source, so
			// nothing is broken for anybody. It is worth a line because a rendition that never comes
			// back means every future request pays the full-size transfer.
			app.Logger.Warn("rebuilding a missing rendition",
				"target", plan.Target.String(), "source", plan.Source.String(), "err", err)
		}
	}()
}

// repairRendition rebuilds one rendition from its source and stores it under its existing ref.
//
// Single-flighted on the target ref, which is the whole point of the function. An album page asks for up
// to sixty thumbnails and a cold cache answers sixty misses at once; without this, sixty goroutines
// decode and resize the same JPEG concurrently, on the one process that is also serving the pages. With
// it, the first arrival does the work and the rest wait on its result.
//
// In-process rather than a lock file on the volume, because this service is single-process **by
// design** — see `docker-compose.prod.yml`, which forbids replicas for two reasons much older than this
// feature (the in-memory PIN store, and projections running as ephemeral consumers with no queue group;
// task 064, PRD 008 §11 Q1). A filesystem marker would be coordinating between processes that cannot
// exist, while adding a TTL to guess wrong, residue to clean up after a crash, and — if it were ever
// placed at the object's own path — a zero-byte file that `Get` would happily serve as an empty image.
func (app *application) repairRendition(ctx context.Context, plan renditionRepair) error {
	if !plan.possible() {
		return errors.New("rendition repair: nothing to rebuild")
	}

	_, err, _ := app.renditionRebuilds.Do(plan.Target.String(), func() (any, error) {
		// Re-check under the flight: by the time this runs, an earlier flight for the same target may
		// already have finished and written it. Cheaper than a decode, and it makes a burst of misses
		// cost one rebuild even when they do not overlap perfectly.
		if ok, err := app.blobs.Exists(ctx, plan.Target); err == nil && ok {
			return nil, nil
		}

		source, err := app.readBlob(ctx, plan.Source)
		if err != nil {
			// Includes the case that matters most: the source is gone too. Then this rendition is not
			// rebuildable and never will be, which is a real outcome rather than an error to retry —
			// the caller has already served what it could.
			return nil, fmt.Errorf("reading the source: %w", err)
		}

		// One thumbnail, no display rendition to speak of and no original: `Prepare` is given the edge
		// as both the display bound and the thumbnail size, and only the thumbnail is used. Same code
		// path as the upload, which is what keeps a rebuilt rendition looking like an uploaded one.
		prepared, err := imaging.Prepare(source, plan.Edge, []int{plan.Edge}, plan.Quality, false)
		if err != nil {
			return nil, fmt.Errorf("re-rendering: %w", err)
		}
		if len(prepared.Thumbs) == 0 || len(prepared.Thumbs[0].Bytes) == 0 {
			return nil, errors.New("re-rendering produced no thumbnail")
		}

		// PutAs, not PutCache: the bytes are byte-different from the ones originally stored, and the
		// whole scheme is that they go back under the *recorded* name anyway. PutAs refuses if that name
		// turns out to belong to an original.
		if err := app.blobs.PutAs(ctx, plan.Target, prepared.Thumbs[0].Bytes); err != nil {
			return nil, err
		}
		app.Logger.Info("rebuilt a missing rendition",
			"target", plan.Target.String(), "source", plan.Source.String(),
			"edge", plan.Edge, "bytes", len(prepared.Thumbs[0].Bytes))
		return nil, nil
	})
	return err
}

// readBlob reads a whole object into memory.
//
// Bounded by maxGlimtUpload rather than unbounded: the source is something this service stored, so it is
// already within its own upload limit, and a limit here means a corrupted or unexpectedly huge object
// cannot turn a background rebuild into the thing that exhausts the process's memory.
func (app *application) readBlob(ctx context.Context, ref blob.Ref) ([]byte, error) {
	rc, err := app.blobs.Get(ctx, ref)
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, io.LimitReader(rc, maxGlimtUpload)); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

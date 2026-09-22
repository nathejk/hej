// Package photobytes brings a patrol photograph's bytes into this app's own blob store (task 361).
//
// # Why this exists at all
//
// The `photographed` event carries **refs, never image data** — a photograph on an append-only log would make
// every replay expensive. The bytes live in foto's content-addressed store, served at `{base}/photos/<ref>`, and
// hq deliberately never proxies them: the ref is the hash of the bytes, so foto's responses are immutable and
// cached forever by a browser, and a proxy would put an uncacheable hop in front of that for nothing.
//
// A diploma is different, because it is rendered **server-side**: the bytes have to be inside this process to be
// embedded in a PDF. The maintainer's instruction was to hold them here — *"include cover photo in own blob
// store, these photos will also be part of the photo gallery shortly"* — so this fetches once and stores, rather
// than reaching for foto on every render.
//
// # Content addressing is what makes this safe
//
// Both stores key objects by the sha256 of their contents, in lowercase hex. So the ref in the event is also the
// key here, and it is **verifiable**: the fetched bytes are hashed and compared before anything is stored. A
// mismatch is refused, which means a compromised or confused upstream cannot substitute an image for a ref this
// app then serves under a name somebody else's link points at. That check is cheap and it is the only reason
// trusting a URL from an event body is defensible.
//
// # What it does not do
//
// No retry loop and no background prefetch. A failure returns an error, the caller omits the photograph, and the
// next request tries again — which is the right shape while the only caller is one diploma. A gallery laying out
// several hundred patrols will want a warm-up pass over refs it knows about; that is a worthwhile addition and
// not this package's business yet.
package photobytes

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"nathejk.dk/internal/blob"
)

// MaxBytes bounds what will be read from foto for one photograph.
//
// 12 MB. A display rendition is a few hundred kilobytes and an unprocessed phone photograph is a handful of
// megabytes, so this is generous — but it is a bound, and the reason there is one is that the response comes
// from another service over the network: without a limit, one confused upstream can exhaust this process's memory
// through an unauthenticated public route.
const MaxBytes = 12 << 20

// FetchTimeout is how long a single fetch may take.
//
// The caller is rendering a page a family is waiting for, and a photograph is decoration on a certificate that is
// worth serving without one. Ten seconds is the same budget `internal/sms` gives its provider.
const FetchTimeout = 10 * time.Second

// ErrUnavailable means the bytes could not be obtained. Callers treat it as "no photograph", never as a failure
// of the thing they were rendering.
var ErrUnavailable = errors.New("photograph bytes unavailable")

// Store is the subset of blob.Store this needs. An interface so a test needs no filesystem.
type Store interface {
	Get(ctx context.Context, ref blob.Ref) (io.ReadCloser, error)
	Exists(ctx context.Context, ref blob.Ref) (bool, error)
	Put(ctx context.Context, data []byte) (blob.Ref, error)
}

// Fetcher reads a photograph's bytes, from the local store when it can and from foto when it must.
type Fetcher struct {
	base   string
	store  Store
	client *http.Client
	logger *slog.Logger

	// inflight holds the fetch currently running for a ref, so concurrent callers wait for it rather than each
	// starting their own. Same reasoning as the merged track's (task 347): on an unauthenticated route, two
	// hundred visitors arriving in the same second all miss a cold store, and nothing else bounds how many
	// requests that turns into against another service.
	mu       sync.Mutex
	inflight map[blob.Ref]*fetch
}

type fetch struct {
	done chan struct{}
	data []byte
	err  error
}

// New returns a Fetcher, or nil when it cannot work.
//
// Nil for an empty base URL or a missing store, rather than a Fetcher that fails every call: the caller already
// has to handle "no photograph", and a nil is a configuration fact that can be logged once at boot instead of an
// error logged per request.
func New(baseURL string, store Store, logger *slog.Logger) *Fetcher {
	if baseURL == "" || store == nil {
		return nil
	}
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &Fetcher{
		base:     strings.TrimSuffix(baseURL, "/"),
		store:    store,
		client:   &http.Client{Timeout: FetchTimeout},
		logger:   logger,
		inflight: map[blob.Ref]*fetch{},
	}
}

// Bytes returns the photograph's bytes for a ref.
//
// Local store first. On a miss it fetches from foto, verifies the hash, stores, and returns — so the second
// caller for the same photograph, and every caller after a restart, is served from disk.
func (f *Fetcher) Bytes(ctx context.Context, ref blob.Ref) ([]byte, error) {
	if f == nil {
		return nil, ErrUnavailable
	}
	// Validated before it reaches a URL path or a filesystem lookup. A ref arrives from an event body, and
	// "../../etc/passwd" is a ref-shaped string.
	//
	// **Stricter than `blob.Ref.Valid`, on purpose.** That accepts uppercase hex; this does not, and the test
	// that found it was passing an uppercase ref straight through to the network. The stores produce lowercase,
	// so an uppercase ref is not something this app wrote — and if it were honoured, identical bytes would live
	// under two names, the uppercase spelling would miss the local store forever, and every request for it would
	// hit foto again. The projection's `validRef` applies the same rule for the same reason.
	if !lowerHexRef(ref) {
		return nil, fmt.Errorf("%w: invalid ref", ErrUnavailable)
	}

	if data, err := f.fromStore(ctx, ref); err == nil {
		return data, nil
	}
	return f.fetchOnce(ctx, ref)
}

func (f *Fetcher) fromStore(ctx context.Context, ref blob.Ref) ([]byte, error) {
	ok, err := f.store.Exists(ctx, ref)
	if err != nil || !ok {
		return nil, ErrUnavailable
	}
	rc, err := f.store.Get(ctx, ref)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(io.LimitReader(rc, MaxBytes))
}

// fetchOnce runs one fetch per ref, however many callers ask at once.
func (f *Fetcher) fetchOnce(ctx context.Context, ref blob.Ref) ([]byte, error) {
	f.mu.Lock()
	if running, ok := f.inflight[ref]; ok {
		f.mu.Unlock()
		select {
		case <-running.done:
			return running.data, running.err
		case <-ctx.Done():
			// This caller gave up; the fetch continues for the others. A photograph fetched for nobody is
			// still a photograph in the store for the next request.
			return nil, ctx.Err()
		}
	}
	running := &fetch{done: make(chan struct{})}
	f.inflight[ref] = running
	f.mu.Unlock()

	running.data, running.err = f.fetchAndStore(ctx, ref)

	f.mu.Lock()
	delete(f.inflight, ref)
	f.mu.Unlock()
	close(running.done)

	return running.data, running.err
}

func (f *Fetcher) fetchAndStore(ctx context.Context, ref blob.Ref) ([]byte, error) {
	url := f.base + "/photos/" + ref.String()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: %s answered %d", ErrUnavailable, url, resp.StatusCode)
	}

	// One byte past the limit, so a response that is exactly at it is not silently truncated into a hash
	// mismatch that reads like tampering.
	data, err := io.ReadAll(io.LimitReader(resp.Body, MaxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	if len(data) > MaxBytes {
		return nil, fmt.Errorf("%w: %s is larger than %d bytes", ErrUnavailable, url, MaxBytes)
	}

	// **The check that makes trusting the URL defensible.** The ref is the hash of the bytes, so this either
	// matches or the bytes are not the photograph the event named. Logged at error level rather than only
	// returned: a mismatch is not an outage, it is either corruption or substitution, and it should be visible
	// even though the caller's correct response is to render without a picture.
	if got := hashOf(data); got != ref {
		f.logger.Error("a photograph's bytes did not match its ref",
			"url", url, "expected", ref.String(), "got", got.String())
		return nil, fmt.Errorf("%w: %s hashed to %s", ErrUnavailable, url, got)
	}

	// **A failed store is not a failed fetch.** The bytes are verified and in hand, so the diploma can carry the
	// photograph; what failed is the caching, which costs a repeat fetch next time rather than the picture. Logged
	// so a broken or full volume surfaces as something other than mysteriously slow renders.
	if stored, err := f.store.Put(ctx, data); err != nil {
		f.logger.Error("storing a fetched photograph", "ref", ref.String(), "err", err)
	} else if stored != ref {
		// Unreachable: Put hashes the same bytes with the same algorithm. Logged rather than returned because if
		// the two stores ever disagree about their hashing, every ref in this app quietly stops meaning what the
		// events say — and that is worth a line in the log even on the request that got usable bytes.
		f.logger.Error("the blob store disagreed about a photograph's hash",
			"ref", ref.String(), "stored", stored.String())
	}
	return data, nil
}

func hashOf(data []byte) blob.Ref {
	sum := sha256.Sum256(data)
	return blob.Ref(hex.EncodeToString(sum[:]))
}

// lowerHexRef reports whether a ref is 64 lowercase hex characters.
//
// See the note in Bytes for why uppercase is rejected even though `blob.Ref.Valid` allows it.
func lowerHexRef(ref blob.Ref) bool {
	if len(ref) != 64 {
		return false
	}
	for _, c := range ref {
		switch {
		case c >= '0' && c <= '9', c >= 'a' && c <= 'f':
		default:
			return false
		}
	}
	return true
}

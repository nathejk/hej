package photobytes

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"nathejk.dk/internal/blob"
)

func refOf(data []byte) blob.Ref {
	sum := sha256.Sum256(data)
	return blob.Ref(hex.EncodeToString(sum[:]))
}

// fotoStub serves objects by ref, the way foto does, and counts requests.
func fotoStub(t *testing.T, objects map[blob.Ref][]byte, hits *int64) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(hits, 1)
		ref := blob.Ref(strings.TrimPrefix(r.URL.Path, "/photos/"))
		data, ok := objects[ref]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write(data)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestBytesFetchesVerifiesAndStores(t *testing.T) {
	data := []byte("not really a jpeg, but hashable")
	ref := refOf(data)
	var hits int64
	srv := fotoStub(t, map[blob.Ref][]byte{ref: data}, &hits)

	store := blob.NewMemoryStore()
	f := New(srv.URL, store, nil)

	got, err := f.Bytes(context.Background(), ref)
	if err != nil {
		t.Fatalf("Bytes: %v", err)
	}
	if string(got) != string(data) {
		t.Errorf("got %q, want %q", got, data)
	}

	// **Stored under the same ref**, which is the property that makes one store's key mean the other's.
	if ok, err := store.Exists(context.Background(), ref); err != nil || !ok {
		t.Errorf("the bytes should be in the local store (ok=%v err=%v)", ok, err)
	}

	// And the second call is served locally: the whole point of holding them here.
	if _, err := f.Bytes(context.Background(), ref); err != nil {
		t.Fatalf("second Bytes: %v", err)
	}
	if hits != 1 {
		t.Errorf("foto was asked %d times, want 1", hits)
	}
}

// **The check that makes trusting a URL from an event body defensible.** If the bytes do not hash to the ref, they
// are not the photograph the event named — corruption or substitution — and nothing is stored under a name
// somebody else's link points at.
func TestBytesThatDoNotMatchTheRefAreRefused(t *testing.T) {
	honest := []byte("the real photograph")
	ref := refOf(honest)
	var hits int64
	// foto serves something else under that ref.
	srv := fotoStub(t, map[blob.Ref][]byte{ref: []byte("a different image entirely")}, &hits)

	store := blob.NewMemoryStore()
	f := New(srv.URL, store, nil)

	if _, err := f.Bytes(context.Background(), ref); !errors.Is(err, ErrUnavailable) {
		t.Errorf("want ErrUnavailable, got %v", err)
	}
	if ok, _ := store.Exists(context.Background(), ref); ok {
		t.Error("mismatched bytes must not be stored")
	}
}

// A ref arrives in an event body and becomes a URL path and a filesystem lookup. `../../` is a ref-shaped string
// only until it is checked.
func TestAnInvalidRefIsRefusedWithoutAsking(t *testing.T) {
	var hits int64
	srv := fotoStub(t, nil, &hits)
	f := New(srv.URL, blob.NewMemoryStore(), nil)

	for _, ref := range []blob.Ref{"", "nope", "../../etc/passwd", blob.Ref(strings.Repeat("A", 64))} {
		if _, err := f.Bytes(context.Background(), ref); !errors.Is(err, ErrUnavailable) {
			t.Errorf("ref %q: want ErrUnavailable, got %v", ref, err)
		}
	}
	if hits != 0 {
		t.Errorf("an invalid ref must not reach foto, got %d requests", hits)
	}
}

// A 404 from foto is "no photograph", not a broken diploma.
func TestAMissingObjectIsUnavailableRatherThanFatal(t *testing.T) {
	var hits int64
	srv := fotoStub(t, map[blob.Ref][]byte{}, &hits)
	f := New(srv.URL, blob.NewMemoryStore(), nil)

	_, err := f.Bytes(context.Background(), refOf([]byte("absent")))
	if !errors.Is(err, ErrUnavailable) {
		t.Errorf("want ErrUnavailable, got %v", err)
	}
}

// **One fetch per ref, however many callers ask at once.** The diploma is on an unauthenticated route, so the
// morning-after burst is the normal case: without this, two hundred visitors are two hundred requests to another
// service for the same immutable object. Measured the same way task 347's burst test measures the merged track.
func TestConcurrentCallersShareOneFetch(t *testing.T) {
	data := []byte("one photograph, many readers")
	ref := refOf(data)
	var hits int64

	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&hits, 1)
		<-release // hold the first fetch open so the others pile up behind it
		_, _ = w.Write(data)
	}))
	t.Cleanup(srv.Close)

	f := New(srv.URL, blob.NewMemoryStore(), nil)

	const callers = 50
	var wg sync.WaitGroup
	errs := make([]error, callers)
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = f.Bytes(context.Background(), ref)
		}(i)
	}

	// Let them arrive, then let the single fetch finish.
	close(release)
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("caller %d: %v", i, err)
		}
	}
	// Not asserted as exactly 1: a caller arriving after the fetch completed and before the next one starts is
	// served from the store, and one that arrives in the gap legitimately starts its own. What must not happen is
	// a request per caller.
	if hits > 5 {
		t.Errorf("%d fetches for %d concurrent callers — the single-flight is not working", hits, callers)
	}
}

// A response larger than the bound is refused rather than read into memory: it arrives from another service over
// the network, on a route anybody can call.
func TestAnOversizedResponseIsRefused(t *testing.T) {
	huge := make([]byte, MaxBytes+1024)
	ref := refOf(huge)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(huge)
	}))
	t.Cleanup(srv.Close)

	f := New(srv.URL, blob.NewMemoryStore(), nil)
	if _, err := f.Bytes(context.Background(), ref); !errors.Is(err, ErrUnavailable) {
		t.Errorf("want ErrUnavailable for an oversized response, got %v", err)
	}
}

// No base URL means the feature is off, and a nil Fetcher must be usable rather than a panic: the caller's
// correct behaviour is identical to "this patrol has no photograph".
func TestANilFetcherIsSafeToCall(t *testing.T) {
	if got := New("", blob.NewMemoryStore(), nil); got != nil {
		t.Error("an empty base URL should give a nil Fetcher")
	}

	var f *Fetcher
	if _, err := f.Bytes(context.Background(), refOf([]byte("x"))); !errors.Is(err, ErrUnavailable) {
		t.Errorf("a nil Fetcher should answer ErrUnavailable, got %v", err)
	}
}

// The local store is consulted before the network, including across a restart — which is what "our own blob
// store" buys: a diploma rendered a year later needs no other service to be alive.
func TestAStoredPhotographNeedsNoNetwork(t *testing.T) {
	data := []byte("already here")
	store := blob.NewMemoryStore()
	ref, err := store.Put(context.Background(), data)
	if err != nil {
		t.Fatalf("Put: %v", err)
	}

	// A base URL that cannot answer, to prove nothing reaches for it.
	f := New("http://127.0.0.1:1/unreachable", store, nil)

	got, err := f.Bytes(context.Background(), ref)
	if err != nil {
		t.Fatalf("Bytes: %v", err)
	}
	if string(got) != string(data) {
		t.Errorf("got %q, want %q", got, data)
	}
}

var _ io.Reader = (*strings.Reader)(nil)

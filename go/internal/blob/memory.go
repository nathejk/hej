package blob

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
)

// MemoryStore keeps objects in memory. For tests, and for running the API with no
// volume configured.
//
// It has no volume and therefore no backup, so the original/cache split cannot matter
// for durability here. It still *records* the class, for one reason: the classification
// is a decision made at each call site in cmd/api, and without somewhere to observe it
// no test could catch a thumbnail being stored as an original or — the costly direction —
// a sole-copy photograph being stored as cache. The production store is exercised by a
// handful of tests; the call sites are exercised by all of them.
type MemoryStore struct {
	mu      sync.RWMutex
	objects map[Ref][]byte
	cached  map[Ref]bool
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{objects: make(map[Ref][]byte), cached: make(map[Ref]bool)}
}

func (s *MemoryStore) Put(ctx context.Context, data []byte) (Ref, error) {
	return s.put(ctx, data, false)
}

func (s *MemoryStore) PutCache(ctx context.Context, data []byte) (Ref, error) {
	return s.put(ctx, data, true)
}

// PutAs writes rebuilt bytes under an existing derived name. See the package comment.
func (s *MemoryStore) PutAs(ctx context.Context, ref Ref, data []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !ref.Valid() {
		return fmt.Errorf("blob: invalid ref %q", string(ref))
	}

	stored := make([]byte, len(data))
	copy(stored, data)

	s.mu.Lock()
	defer s.mu.Unlock()
	// The same refusal FileStore makes, and for the same reason. Mirrored rather than
	// skipped because this is the store every cmd/api test runs against, so a repair path
	// that muddled its source and target refs would otherwise pass the whole suite.
	if wasCached, known := s.cached[ref]; known && !wasCached {
		return fmt.Errorf("blob: refusing to rebuild %s: it names an original, not a rendition", ref)
	}
	s.objects[ref] = stored
	s.cached[ref] = true
	return nil
}

func (s *MemoryStore) put(ctx context.Context, data []byte, cache bool) (Ref, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	ref := ComputeRef(data)

	// Copy: the caller may reuse or mutate its buffer, and a content-addressed
	// store whose contents change out from under its own hash is not one.
	stored := make([]byte, len(data))
	copy(stored, data)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.objects[ref] = stored
	// Mirrors FileStore.put's promotion rule: once any caller has claimed these bytes as
	// an original, a later cache write must not downgrade them.
	if !cache {
		s.cached[ref] = false
	} else if _, seen := s.cached[ref]; !seen {
		s.cached[ref] = true
	}
	return ref, nil
}

// IsCached reports whether the object was stored as a derived rendition. Test helper.
func (s *MemoryStore) IsCached(ref Ref) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cached[ref]
}

func (s *MemoryStore) Get(ctx context.Context, ref Ref) (io.ReadCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !ref.Valid() {
		return nil, ErrNotFound
	}
	s.mu.RLock()
	data, ok := s.objects[ref]
	s.mu.RUnlock()
	if !ok {
		return nil, ErrNotFound
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (s *MemoryStore) Exists(ctx context.Context, ref Ref) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	s.mu.RLock()
	_, ok := s.objects[ref]
	s.mu.RUnlock()
	return ok, nil
}

func (s *MemoryStore) Delete(ctx context.Context, ref Ref) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	delete(s.objects, ref)
	delete(s.cached, ref)
	s.mu.Unlock()
	return nil
}

// Len reports how many objects are stored. Test helper.
func (s *MemoryStore) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.objects)
}

var _ Store = (*MemoryStore)(nil)

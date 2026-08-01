// Package push stores Web Push subscriptions so event updates can be delivered
// later (delivery/fan-out is a separate PRD). The skeleton uses an in-memory
// store; a persistent store can implement the same interface.
package push

import (
	"sync"
	"time"
)

// Subscription is a client's Web Push subscription, tied to a user.
type Subscription struct {
	UserID    string
	Endpoint  string
	P256dh    string
	Auth      string
	CreatedAt time.Time
}

// Store persists push subscriptions.
type Store interface {
	// Save stores (or replaces) a subscription. Idempotent per (user, endpoint).
	Save(sub Subscription)
	// All returns every stored subscription.
	All() []Subscription
}

// MemoryStore is an in-memory Store keyed by user id + endpoint.
type MemoryStore struct {
	mu    sync.Mutex
	byKey map[string]Subscription
}

// NewMemoryStore returns an empty in-memory subscription store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{byKey: make(map[string]Subscription)}
}

func (s *MemoryStore) Save(sub Subscription) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byKey[sub.UserID+"|"+sub.Endpoint] = sub
}

func (s *MemoryStore) All() []Subscription {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Subscription, 0, len(s.byKey))
	for _, sub := range s.byKey {
		out = append(out, sub)
	}
	return out
}

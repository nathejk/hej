package commands

import (
	"sync/atomic"

	"github.com/jrgensen/cqrs"
)

// PublisherHolder carries a publisher that may arrive after startup.
//
// The broker is connected in the background (PRD 008 §6: startup must not block on
// it), so at the moment handlers are wired there may be no publisher yet. Handlers
// hold this instead of a publisher so they do not need to be re-wired when one
// appears.
//
// atomic.Pointer rather than a mutex because the read is on every write request
// and the write happens once or twice in a process lifetime.
type PublisherHolder struct {
	p atomic.Pointer[cqrs.Publisher]
}

// NewPublisherHolder returns an empty holder.
func NewPublisherHolder() *PublisherHolder { return &PublisherHolder{} }

// Set installs a publisher, replacing any previous one.
//
// A nil publisher clears the holder. Note the interface value is stored behind a
// pointer specifically so a typed-nil publisher cannot be mistaken for a live one:
// callers pass nil explicitly to clear.
func (h *PublisherHolder) Set(p cqrs.Publisher) {
	if h == nil {
		return
	}
	if p == nil {
		h.p.Store(nil)
		return
	}
	h.p.Store(&p)
}

// Get returns the current publisher, or nil if none is installed.
func (h *PublisherHolder) Get() cqrs.Publisher {
	if h == nil {
		return nil
	}
	if p := h.p.Load(); p != nil {
		return *p
	}
	return nil
}

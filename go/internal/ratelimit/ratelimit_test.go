package ratelimit

import (
	"testing"
	"time"
)

func TestLimiter_AllowsUpToLimitThenBlocks(t *testing.T) {
	now := time.Now()
	l := New(3, time.Minute)
	l.now = func() time.Time { return now }

	for i := 0; i < 3; i++ {
		if !l.Allow("ip") {
			t.Fatalf("event %d should be allowed", i)
		}
	}
	if l.Allow("ip") {
		t.Fatal("4th event should be blocked")
	}
}

func TestLimiter_WindowResets(t *testing.T) {
	now := time.Now()
	l := New(1, time.Minute)
	l.now = func() time.Time { return now }

	if !l.Allow("ip") {
		t.Fatal("first event should be allowed")
	}
	if l.Allow("ip") {
		t.Fatal("second event within window should be blocked")
	}
	now = now.Add(time.Minute + time.Second)
	if !l.Allow("ip") {
		t.Fatal("event after window should be allowed")
	}
}

func TestLimiter_KeysAreIndependent(t *testing.T) {
	l := New(1, time.Minute)
	if !l.Allow("a") {
		t.Fatal("key a first event should be allowed")
	}
	if !l.Allow("b") {
		t.Fatal("key b first event should be allowed")
	}
}

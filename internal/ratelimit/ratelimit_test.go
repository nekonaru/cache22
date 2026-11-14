package ratelimit

import (
	"testing"
	"time"
)

func TestBurstThenDeny(t *testing.T) {
	l := New(1, 3) // 1 token/s, burst 3
	for i := 0; i < 3; i++ {
		if !l.Allow("ip") {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}
	if l.Allow("ip") {
		t.Fatal("burst exhausted: request should be denied")
	}
}

func TestRefill(t *testing.T) {
	l := New(2, 2) // 2 tokens/s
	base := time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC)
	l.now = func() time.Time { return base }
	l.Allow("ip")
	l.Allow("ip")
	if l.Allow("ip") {
		t.Fatal("should be denied at burst limit")
	}
	l.now = func() time.Time { return base.Add(500 * time.Millisecond) } // +1 token
	if !l.Allow("ip") {
		t.Fatal("one token should have refilled")
	}
}

func TestKeysAreIndependent(t *testing.T) {
	l := New(0.001, 1)
	if !l.Allow("a") {
		t.Fatal()
	}
	if l.Allow("a") {
		t.Fatal("a should be limited")
	}
	if !l.Allow("b") {
		t.Fatal("b has its own bucket")
	}
}

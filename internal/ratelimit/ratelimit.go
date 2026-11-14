// Package ratelimit implements a small per-IP token bucket, self-contained
// (no external dependencies).
package ratelimit

import (
	"sync"
	"time"
)

// Limiter is a token bucket keyed by an arbitrary string (usually the
// client IP). It is safe for concurrent use.
type Limiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	rate    float64 // tokens per second refill
	burst   float64 // bucket capacity
	now     func() time.Time
	idleTTL time.Duration
	lastGC  time.Time
}

type bucket struct {
	tokens float64
	last   time.Time
}

// New returns a Limiter that allows burst requests initially and refills
// at rate tokens per second per key.
func New(rate, burst float64) *Limiter {
	return &Limiter{
		buckets: make(map[string]*bucket),
		rate:    rate,
		burst:   burst,
		now:     time.Now,
		idleTTL: 10 * time.Minute,
	}
}

// Allow consumes one token for key, reporting whether the request may
// proceed.
func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	l.gc(now)
	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{tokens: l.burst, last: now}
		l.buckets[key] = b
	}
	elapsed := now.Sub(b.last).Seconds()
	b.tokens += elapsed * l.rate
	if b.tokens > l.burst {
		b.tokens = l.burst
	}
	b.last = now
	if b.tokens < 1 {
		return false
	}
	b.tokens -= 1
	return true
}

// gc drops buckets that have been idle longer than idleTTL, but only runs
// at most once per minute to keep Allow cheap.
func (l *Limiter) gc(now time.Time) {
	if now.Sub(l.lastGC) < time.Minute {
		return
	}
	l.lastGC = now
	for k, b := range l.buckets {
		if now.Sub(b.last) > l.idleTTL {
			delete(l.buckets, k)
		}
	}
}

func (l *Limiter) Len() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.buckets)
}

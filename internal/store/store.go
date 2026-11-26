// Package store defines the storage contract for one-time drops and ships
// the built-in in-memory implementation.
package store

import (
	"crypto/subtle"
	"errors"
	"sync"
	"time"
)

// ErrFull reports that the store reached its configured memory budget.
var ErrFull = errors.New("store: capacity reached")

// KindText and KindFile are the only payload kinds accepted by the API.
const (
	KindText  = "text"
	KindFile  = "file"
	KindFiles = "files"
)

// Drop is a single stored secret. Payload is stored exactly as the client
// sent it (base64url); the server never inspects or transforms it.
type Drop struct {
	ID        string
	Payload   string
	Kind      string
	Meta      []byte // arbitrary small JSON object, opaque to the server; may be nil
	ReadsLeft int
	ExpiresAt time.Time
	CreatedAt time.Time

	// OwnerHash is SHA-256 of a high-entropy control token. The raw token
	// is returned only at create time and never stored or logged.
	OwnerHash []byte

	// Extended records that the owner already used the one-time TTL extension.
	Extended bool

	// size tracks decoded payload bytes for the memory budget.
	size int
}

// Store is the persistence contract. Consume must be atomic so concurrent
// readers of a single-read drop observe exactly one success.
type Store interface {
	Put(d Drop) error
	// Consume atomically fetches d, decrements its remaining reads, and
	// deletes it when the counter reaches zero or the drop is expired.
	// Returns ok=false when the drop is missing, expired, or exhausted.
	Consume(id string, now time.Time) (Drop, bool)
	// OwnerGet returns a drop only when id and ownerHash match. It does not
	// consume the drop.
	OwnerGet(id string, ownerHash []byte, now time.Time) (Drop, bool)
	// OwnerDelete removes a drop only when id and ownerHash match.
	OwnerDelete(id string, ownerHash []byte, now time.Time) bool
	// OwnerUpdate replaces a drop only when id and ownerHash match and the
	// drop is still live. It is used for the single TTL extension.
	OwnerUpdate(d Drop, ownerHash []byte, now time.Time) bool
	// Sweep permanently removes expired drops and reports how many.
	Sweep(now time.Time) int
	// SweepIDs removes expired drops and returns their ids for notifications.
	SweepIDs(now time.Time) []string
}

// Memory is an in-memory Store guarded by a single mutex.
//
// A plain map + mutex beats sync.Map here because Consume needs a
// read-modify-write cycle (decrement remaining reads, maybe delete) that
// must be atomic.
type Memory struct {
	mu       sync.Mutex
	drops    map[string]*Drop
	used     int64
	maxBytes int64
}

// NewMemory returns a Memory store bounded to maxBytes of decoded payload
// data. maxBytes <= 0 means unbounded (only used in tests).
func NewMemory(maxBytes int64) *Memory {
	return &Memory{drops: make(map[string]*Drop), maxBytes: maxBytes}
}

// Put stores a drop. The decoded payload size counts against the budget;
// Put fails with ErrFull when it would be exceeded.
func (m *Memory) Put(d Drop) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.maxBytes > 0 && m.used+int64(d.size) > m.maxBytes {
		return ErrFull
	}
	cp := d
	m.drops[d.ID] = &cp
	m.used += int64(d.size)
	return nil
}

// NewDrop builds a Drop, stamping sizes for the budget.
func NewDrop(id, payload, kind string, meta []byte, reads int, now time.Time, ttl time.Duration, payloadBytes int) Drop {
	return Drop{
		ID:        id,
		Payload:   payload,
		Kind:      kind,
		Meta:      append([]byte(nil), meta...),
		ReadsLeft: reads,
		ExpiresAt: now.Add(ttl),
		CreatedAt: now,
		size:      payloadBytes,
	}
}

// Consume implements atomic read-and-decrement.
func (m *Memory) Consume(id string, now time.Time) (Drop, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.drops[id]
	if !ok {
		return Drop{}, false
	}
	if !now.Before(d.ExpiresAt) {
		m.remove(id)
		return Drop{}, false
	}
	d.ReadsLeft--
	out := *d
	if d.ReadsLeft <= 0 {
		m.remove(id)
	}
	return out, true
}

// Sweep removes all expired drops and returns the count removed.
func (m *Memory) Sweep(now time.Time) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for id, d := range m.drops {
		if !now.Before(d.ExpiresAt) {
			m.remove(id)
			n++
		}
	}
	return n
}

func ownerMatch(d *Drop, ownerHash []byte) bool {
	return len(ownerHash) == 32 && subtle.ConstantTimeCompare(d.OwnerHash, ownerHash) == 1
}

func (m *Memory) SweepIDs(now time.Time) []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	var ids []string
	for id, d := range m.drops {
		if !now.Before(d.ExpiresAt) {
			ids = append(ids, id)
			m.remove(id)
		}
	}
	return ids
}

func (m *Memory) OwnerGet(id string, ownerHash []byte, now time.Time) (Drop, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.drops[id]
	if !ok || !now.Before(d.ExpiresAt) || !ownerMatch(d, ownerHash) {
		return Drop{}, false
	}
	out := *d
	return out, true
}

func (m *Memory) OwnerDelete(id string, ownerHash []byte, now time.Time) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.drops[id]
	if !ok || !now.Before(d.ExpiresAt) || !ownerMatch(d, ownerHash) {
		return false
	}
	m.remove(id)
	return true
}

func (m *Memory) OwnerUpdate(d Drop, ownerHash []byte, now time.Time) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	cur, ok := m.drops[d.ID]
	if !ok || !now.Before(cur.ExpiresAt) || !ownerMatch(cur, ownerHash) || cur.Extended || cur.ReadsLeft != d.ReadsLeft {
		return false
	}
	cp := d
	m.drops[d.ID] = &cp
	return true
}

// Len reports the number of stored drops (used by tests and metrics).
func (m *Memory) Len() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.drops)
}

// UsedBytes reports the decoded payload bytes currently held.
func (m *Memory) UsedBytes() int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.used
}

// remove deletes id and credits its bytes back. Caller must hold mu.
func (m *Memory) remove(id string) {
	if d, ok := m.drops[id]; ok {
		m.used -= int64(d.size)
		delete(m.drops, id)
	}
}

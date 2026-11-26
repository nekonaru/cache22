package store

import (
	"testing"
	"time"
)

func FuzzValidID(f *testing.F) {
	f.Add("ABCDEFGHIJKLMNOPQRSTUVWXYZ")
	f.Add("short")
	f.Add("")
	f.Fuzz(func(t *testing.T, id string) {
		_ = validIDForTest(id)
	})
}

func TestOwnerGetMalformedHashes(t *testing.T) {
	// OwnerGet must never return ok for malformed hashes.
	m := NewMemory(0)
	d := newTestDrop("ABCDEFGHIJKLMNOPQRSTUVWXYZ", 1, time.Hour)
	d.OwnerHash = make([]byte, 32)
	_ = m.Put(d)
	for _, h := range [][]byte{nil, {}, make([]byte, 31), make([]byte, 33), make([]byte, 32)} {
		if _, ok := m.OwnerGet(d.ID, h, time.Now()); ok && len(h) != 32 {
			t.Fatalf("malformed hash accepted: %d bytes", len(h))
		}
	}
}

func validIDForTest(id string) bool {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	if len(id) != 22 {
		return false
	}
	for _, r := range id {
		if !containsRune(alphabet, r) {
			return false
		}
	}
	return true
}

func containsRune(s string, r rune) bool {
	for _, c := range s {
		if c == r {
			return true
		}
	}
	return false
}

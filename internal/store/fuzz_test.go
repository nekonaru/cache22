package store

import (
	"testing"
	"time"
)

func TestOwnerGetMalformedHashes(t *testing.T) {
	m := NewMemory(0)
	d := newTestDrop("ABCDEFGHIJKLMNOPQRSTUVWXYZ", 1, time.Hour)
	d.OwnerHash = make([]byte, 32)
	_ = m.Put(d)
	for _, h := range [][]byte{nil, make([]byte, 31), make([]byte, 33), make([]byte, 32)} {
		if _, ok := m.OwnerGet(d.ID, h, t0.Add(time.Minute)); ok && len(h) != 32 {
			t.Fatalf("malformed hash accepted: %d bytes", len(h))
		}
	}
}

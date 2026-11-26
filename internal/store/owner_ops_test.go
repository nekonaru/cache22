package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestOwnerOperationsMemory(t *testing.T) {
	m := NewMemory(0)
	now := t0.Add(time.Minute)
	hash := make([]byte, 32)
	for i := range hash {
		hash[i] = byte(i)
	}
	d := newTestDrop("ABCDEFGHIJKLMNOPQRSTUVWXYZ", 2, time.Hour)
	d.OwnerHash = append([]byte(nil), hash...)
	if err := m.Put(d); err != nil {
		t.Fatal(err)
	}
	if _, ok := m.OwnerGet(d.ID, hash, now); !ok {
		t.Fatal("OwnerGet failed")
	}
	d.ExpiresAt = now.Add(2 * time.Hour)
	d.Extended = true
	if !m.OwnerUpdate(d, hash, now) {
		t.Fatal("OwnerUpdate failed")
	}
	got, ok := m.OwnerGet(d.ID, hash, now)
	if !ok || !got.Extended || !got.ExpiresAt.Equal(d.ExpiresAt) {
		t.Fatalf("bad updated drop: %+v", got)
	}
	if !m.OwnerDelete(d.ID, hash, now) {
		t.Fatal("OwnerDelete failed")
	}
	if m.OwnerDelete(d.ID, hash, now) {
		t.Fatal("second OwnerDelete must fail")
	}
	if m.Len() != 0 || m.UsedBytes() != 0 {
		t.Fatalf("store not empty: len=%d used=%d", m.Len(), m.UsedBytes())
	}
}

func TestOwnerOperationsBolt(t *testing.T) {
	dir := t.TempDir()
	b, err := NewBolt(filepath.Join(dir, "owner.db"), 0)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = b.Close() }()
	now := t0.Add(time.Minute)
	hash := make([]byte, 32)
	for i := range hash {
		hash[i] = byte(i + 1)
	}
	d := newTestDrop("ZYXWVUTSRQPONMLKJIHGFEDCBA", 2, time.Hour)
	d.OwnerHash = append([]byte(nil), hash...)
	if err := b.Put(d); err != nil {
		t.Fatal(err)
	}
	if _, ok := b.OwnerGet(d.ID, hash, now); !ok {
		t.Fatal("bolt OwnerGet failed")
	}
	d.ExpiresAt = now.Add(2 * time.Hour)
	d.Extended = true
	if !b.OwnerUpdate(d, hash, now) {
		t.Fatal("bolt OwnerUpdate failed")
	}
	got, ok := b.OwnerGet(d.ID, hash, now)
	if !ok || !got.Extended {
		t.Fatalf("bad bolt updated drop: %+v", got)
	}
	backup := filepath.Join(dir, "backup.db")
	if err := b.Backup(backup); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(backup); err != nil {
		t.Fatal(err)
	}
	if !b.OwnerDelete(d.ID, hash, now) {
		t.Fatal("bolt OwnerDelete failed")
	}
	if b.OwnerDelete(d.ID, hash, now) {
		t.Fatal("second bolt OwnerDelete must fail")
	}
	if ids := b.SweepIDs(now.Add(3 * time.Hour)); len(ids) != 0 {
		t.Fatalf("unexpected sweep: %v", ids)
	}
}

func TestSweepIDsMemory(t *testing.T) {
	m := NewMemory(0)
	now := t0.Add(time.Minute)
	d := newTestDrop("AAAAAAAAAAAAAAAAAAAAAA", 1, time.Hour)
	if err := m.Put(d); err != nil {
		t.Fatal(err)
	}
	if ids := m.SweepIDs(now); len(ids) != 0 {
		t.Fatalf("unexpected sweep: %v", ids)
	}
	if ids := m.SweepIDs(now.Add(2 * time.Hour)); len(ids) != 1 || ids[0] != d.ID {
		t.Fatalf("sweep ids = %v", ids)
	}
}

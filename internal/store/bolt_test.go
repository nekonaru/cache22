package store

import (
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func openTestBolt(t *testing.T) (*Bolt, string) {
	t.Helper()
	dir := t.TempDir()
	b, err := NewBolt(filepath.Join(dir, "test.db"), 0) // unbounded for these tests
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = b.Close() })
	return b, b.Path()
}

func TestBoltPutConsume(t *testing.T) {
	b, _ := openTestBolt(t)
	if err := b.Put(newTestDrop("a", 1, time.Hour)); err != nil {
		t.Fatal(err)
	}
	d, ok := b.Consume("a", t0.Add(time.Minute))
	if !ok {
		t.Fatal("first consume should succeed")
	}
	if d.Payload != "cGF5bG9hZA" {
		t.Fatalf("payload = %q", d.Payload)
	}
	if _, ok := b.Consume("a", t0.Add(time.Minute)); ok {
		t.Fatal("second consume must fail")
	}
}

func TestBoltPersistAcrossReopen(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "persist.db")
	b1, err := NewBolt(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := b1.Put(newTestDrop("keep", 1, time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := b1.Close(); err != nil {
		t.Fatal(err)
	}

	b2, err := NewBolt(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = b2.Close() }()
	d, ok := b2.Consume("keep", t0.Add(time.Minute))
	if !ok {
		t.Fatal("drop must survive restart")
	}
	if d.Payload != "cGF5bG9hZA" {
		t.Fatalf("payload = %q", d.Payload)
	}
}

func TestBoltConsumeExpired(t *testing.T) {
	b, _ := openTestBolt(t)
	_ = b.Put(newTestDrop("x", 1, time.Hour))
	if _, ok := b.Consume("x", t0.Add(2*time.Hour)); ok {
		t.Fatal("expired drop must not be readable")
	}
	// The expired record is deleted in the same write transaction, so a retry
	// hits the real file state too, not just the in-memory map.
	if _, ok := b.Consume("x", t0.Add(2*time.Hour)); ok {
		t.Fatal("expired drop must be deleted from the file")
	}
}

func TestBoltSweep(t *testing.T) {
	b, _ := openTestBolt(t)
	_ = b.Put(newTestDrop("keep", 1, 24*time.Hour))
	_ = b.Put(newTestDrop("old1", 1, time.Minute))
	_ = b.Put(newTestDrop("old2", 1, time.Minute))
	if n := b.Sweep(t0.Add(2 * time.Minute)); n != 2 {
		t.Fatalf("sweep = %d, want 2", n)
	}
}

func TestBoltMultiRead(t *testing.T) {
	b, _ := openTestBolt(t)
	_ = b.Put(newTestDrop("m", 3, time.Hour))
	for i := 0; i < 3; i++ {
		if _, ok := b.Consume("m", t0.Add(time.Minute)); !ok {
			t.Fatalf("read %d of 3 should succeed", i+1)
		}
	}
	if _, ok := b.Consume("m", t0.Add(time.Minute)); ok {
		t.Fatal("4th read must fail")
	}
}

func TestBoltCapacity(t *testing.T) {
	dir := t.TempDir()
	b, err := NewBolt(filepath.Join(dir, "cap.db"), 10) // 10 bytes budget, 7 per drop
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = b.Close() }()
	if err := b.Put(newTestDrop("a", 1, time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := b.Put(newTestDrop("b", 1, time.Hour)); err != ErrFull {
		t.Fatalf("err = %v, want ErrFull", err)
	}
}

func TestBoltConsumeConcurrency(t *testing.T) {
	b, _ := openTestBolt(t)
	_ = b.Put(newTestDrop("race", 1, time.Hour))
	const n = 100
	var wg sync.WaitGroup
	var mu sync.Mutex
	successes := 0
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, ok := b.Consume("race", t0); ok {
				mu.Lock()
				successes++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if successes != 1 {
		t.Fatalf("successes = %d, want exactly 1", successes)
	}
}

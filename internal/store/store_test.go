package store

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

var t0 = time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)

func newTestDrop(id string, reads int, ttl time.Duration) Drop {
	return NewDrop(id, "cGF5bG9hZA", KindText, nil, reads, t0, ttl, 7)
}

func TestPutConsumeOnce(t *testing.T) {
	m := NewMemory(0)
	if err := m.Put(newTestDrop("a", 1, time.Hour)); err != nil {
		t.Fatal(err)
	}
	d, ok := m.Consume("a", t0.Add(time.Minute))
	if !ok {
		t.Fatal("first consume should succeed")
	}
	if d.Payload != "cGF5bG9hZA" || d.Kind != KindText {
		t.Fatalf("unexpected drop %+v", d)
	}
	if _, ok := m.Consume("a", t0.Add(time.Minute)); ok {
		t.Fatal("second consume must fail")
	}
	if m.Len() != 0 {
		t.Fatal("drop must be deleted after final read")
	}
}

func TestConsumeMultiRead(t *testing.T) {
	m := NewMemory(0)
	_ = m.Put(newTestDrop("m", 3, time.Hour))
	for i := 0; i < 3; i++ {
		if _, ok := m.Consume("m", t0.Add(time.Minute)); !ok {
			t.Fatalf("read %d of 3 should succeed", i+1)
		}
	}
	if _, ok := m.Consume("m", t0.Add(time.Minute)); ok {
		t.Fatal("4th read must fail")
	}
}

func TestConsumeExpired(t *testing.T) {
	m := NewMemory(0)
	_ = m.Put(newTestDrop("x", 1, time.Hour))
	if _, ok := m.Consume("x", t0.Add(2*time.Hour)); ok {
		t.Fatal("expired drop must not be readable")
	}
	if m.Len() != 0 {
		t.Fatal("expired drop must be evicted on consume")
	}
}

func TestSweep(t *testing.T) {
	m := NewMemory(0)
	_ = m.Put(newTestDrop("keep", 1, 24*time.Hour))
	_ = m.Put(newTestDrop("old1", 1, time.Minute))
	_ = m.Put(newTestDrop("old2", 1, time.Minute))
	if n := m.Sweep(t0.Add(2 * time.Minute)); n != 2 {
		t.Fatalf("sweep = %d, want 2", n)
	}
	if m.Len() != 1 {
		t.Fatal("fresh drop must survive sweep")
	}
}

func TestCapacityEvictsBudget(t *testing.T) {
	m := NewMemory(10) // 10 bytes budget, each drop is 7 bytes
	if err := m.Put(newTestDrop("a", 1, time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := m.Put(newTestDrop("b", 1, time.Hour)); !errors.Is(err, ErrFull) {
		t.Fatalf("err = %v, want ErrFull", err)
	}
	_, _ = m.Consume("a", t0)
	if err := m.Put(newTestDrop("b", 1, time.Hour)); err != nil {
		t.Fatalf("budget should be reclaimed after consume: %v", err)
	}
}

func TestConsumeConcurrency(t *testing.T) {
	m := NewMemory(0)
	_ = m.Put(newTestDrop("race", 1, time.Hour))
	const n = 100
	var wg sync.WaitGroup
	var mu sync.Mutex
	successes := 0
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if _, ok := m.Consume("race", t0); ok {
				mu.Lock()
				successes++
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()
	if successes != 1 {
		t.Fatalf("successes = %d, want exactly 1", successes)
	}
}

func TestPayloadStoredVerbatim(t *testing.T) {
	m := NewMemory(0)
	payload := "SGVsbG8tV29ybGRfMTIzNDU2Nzg" // arbitrary base64url
	_ = m.Put(NewDrop("v", payload, KindFile, []byte(`{"has_passphrase":true}`), 1, t0, time.Hour, 17))
	d, ok := m.Consume("v", t0)
	if !ok {
		t.Fatal()
	}
	if d.Payload != payload {
		t.Fatal("payload was modified")
	}
	if string(d.Meta) != `{"has_passphrase":true}` {
		t.Fatal("meta was modified")
	}
}

func ExampleMemory_Consume() {
	m := NewMemory(0)
	_ = m.Put(newTestDrop("demo", 1, time.Hour))
	d, ok := m.Consume("demo", t0.Add(time.Second))
	fmt.Println(ok, d.Kind)
	// Output: true text
}

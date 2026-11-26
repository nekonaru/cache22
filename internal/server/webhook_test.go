package server

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/nekonaru/cache22/internal/webhook"
)

// fakeNotifier records events sent through the server.
type fakeNotifier struct {
	mu     sync.Mutex
	events []webhook.Event
}

func (f *fakeNotifier) Send(ctx context.Context, ev webhook.Event) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, ev)
	return nil
}

func (f *fakeNotifier) countEvent(event string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, ev := range f.events {
		if ev.Event == event {
			n++
		}
	}
	return n
}

func (f *fakeNotifier) lastEvent(event string) webhook.Event {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := len(f.events) - 1; i >= 0; i-- {
		if f.events[i].Event == event {
			return f.events[i]
		}
	}
	return webhook.Event{}
}

func (f *fakeNotifier) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.events)
}

func (f *fakeNotifier) last() webhook.Event {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.events) == 0 {
		return webhook.Event{}
	}
	return f.events[len(f.events)-1]
}

func waitForEvents(n *fakeNotifier, want int) {
	deadline := time.Now().Add(time.Second)
	for n.count() < want+1 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
}

func TestWebhookEventFiredOnRead(t *testing.T) {
	s := testServer(t)
	notifier := &fakeNotifier{}
	s.Webhook = notifier

	rr := createDrop(t, s, `{"payload":"`+validPayload+`","ttl":"1h","kind":"text"}`)
	var cr createResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &cr); err != nil {
		t.Fatal(err)
	}
	do(t, s, "POST", "/api/drops/"+cr.ID+"/read", nil, nil)
	waitForEvents(notifier, 1)
	got := notifier.lastEvent("drop.read")
	if got.Event != "drop.read" || got.Kind != "text" || got.ID != cr.ID || got.ReadsLeft != 0 {
		t.Fatalf("event mismatch: %+v", got)
	}
}

func TestTelegramEventFiredOnRead(t *testing.T) {
	s := testServer(t)
	notifier := &fakeNotifier{}
	s.Telegram = notifier

	rr := createDrop(t, s, `{"payload":"`+validPayload+`","ttl":"1h","kind":"text"}`)
	var cr createResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &cr); err != nil {
		t.Fatal(err)
	}
	do(t, s, "POST", "/api/drops/"+cr.ID+"/read", nil, nil)
	waitForEvents(notifier, 1)
	got := notifier.lastEvent("drop.read")
	if got.Event != "drop.read" || got.Kind != "text" || got.ID != cr.ID || got.ReadsLeft != 0 {
		t.Fatalf("event mismatch: %+v", got)
	}
}

func TestWebhookAndTelegramBothFiredTogether(t *testing.T) {
	s := testServer(t)
	wh := &fakeNotifier{}
	tn := &fakeNotifier{}
	s.Webhook = wh
	s.Telegram = tn

	rr := createDrop(t, s, `{"payload":"`+validPayload+`","ttl":"1h","kind":"text"}`)
	var cr createResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &cr); err != nil {
		t.Fatal(err)
	}
	do(t, s, "POST", "/api/drops/"+cr.ID+"/read", nil, nil)
	waitForEvents(wh, 1)
	waitForEvents(tn, 1)
	if wh.countEvent("drop.read") != 1 || tn.countEvent("drop.read") != 1 {
		t.Fatalf("wh=%d tn=%d, want 1 each", wh.countEvent("drop.read"), tn.countEvent("drop.read"))
	}
}

func TestNoWebhookByDefault(t *testing.T) {
	s := testServer(t)
	rr := createDrop(t, s, `{"payload":"`+validPayload+`","ttl":"1h","kind":"text"}`)
	var cr createResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &cr); err != nil {
		t.Fatal(err)
	}
	if rr := do(t, s, "POST", "/api/drops/"+cr.ID+"/read", nil, nil); rr.Code != 200 {
		t.Fatalf("read status = %d", rr.Code)
	}
}

func TestWebhookEventIncludesFinalRead(t *testing.T) {
	s := testServer(t)
	notifier := &fakeNotifier{}
	s.Webhook = notifier
	rr := createDrop(t, s, `{"payload":"`+validPayload+`","ttl":"1h","kind":"text","max_reads":2}`)
	var cr createResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &cr); err != nil {
		t.Fatal(err)
	}
	do(t, s, "POST", "/api/drops/"+cr.ID+"/read", nil, nil)
	do(t, s, "POST", "/api/drops/"+cr.ID+"/read", nil, nil)
	waitForEvents(notifier, 2)
	if got := notifier.countEvent("drop.read"); got != 2 {
		t.Fatalf("events = %d, want 2", got)
	}
	// The two reads race; check the multiset, not the order.
	notifier.mu.Lock()
	var lefts []int
	for _, ev := range notifier.events {
		if ev.Event == "drop.read" {
			lefts = append(lefts, ev.ReadsLeft)
		}
	}
	notifier.mu.Unlock()
	count1, count0 := 0, 0
	for _, l := range lefts {
		if l == 1 {
			count1++
		} else if l == 0 {
			count0++
		}
	}
	if count1 != 1 || count0 != 1 {
		t.Fatalf("ReadsLeft values = %v, want one 1 and one 0", lefts)
	}
}

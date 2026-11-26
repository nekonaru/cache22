package server

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nekonaru/cache22/internal/webhook"
)

func TestMetricsAndHealthJSON(t *testing.T) {
	s := testServer(t)
	s.cfg.MetricsToken = "metrics-secret"
	rr := do(t, s, "GET", "/metrics", nil, map[string]string{"Authorization": "Bearer metrics-secret"})
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), "cache22_drops_created_total") {
		t.Fatalf("metrics = %d %s", rr.Code, rr.Body.String())
	}
	bad := do(t, s, "GET", "/metrics", nil, nil)
	if bad.Code != 404 {
		t.Fatalf("metrics without token = %d", bad.Code)
	}
	hj := do(t, s, "GET", "/healthz?format=json", nil, nil)
	if hj.Code != 200 {
		t.Fatalf("health json = %d", hj.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(hj.Body.Bytes(), &body); err != nil || body["status"] != "ok" {
		t.Fatalf("health body = %s, err = %v", hj.Body.String(), err)
	}
}

func TestOriginCheck(t *testing.T) {
	s := testServer(t)
	s.cfg.RequireOrigin = true
	s.cfg.BaseURL = "https://secret.example"
	rr := do(t, s, "POST", "/api/drops", strings.NewReader(`{"payload":"`+validPayload+`","ttl":"1h","kind":"text"}`),
		map[string]string{"Origin": "https://evil.example", "Content-Type": "application/json"})
	if rr.Code != 404 {
		t.Fatalf("bad origin = %d, want 404", rr.Code)
	}
}

func TestTTLAllowlistAndOptions(t *testing.T) {
	s := testServer(t)
	s.cfg.TTLAllowlist = "30m,1h"
	if _, ok := s.allowedTTL()["30m"]; !ok {
		t.Fatal("30m missing")
	}
	if _, ok := s.allowedTTL()["7d"]; ok {
		t.Fatal("7d should be disabled")
	}
	page := do(t, s, "GET", "/", nil, nil)
	if !strings.Contains(page.Body.String(), `value="30m"`) {
		t.Fatal("page must expose configured TTL")
	}
}

type eventNotifier struct {
	mu     sync.Mutex
	events []webhook.Event
}

func (n *eventNotifier) Send(ctx context.Context, ev webhook.Event) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.events = append(n.events, ev)
	return nil
}

func (n *eventNotifier) snapshot() []webhook.Event {
	n.mu.Lock()
	defer n.mu.Unlock()
	return append([]webhook.Event(nil), n.events...)
}

func TestNotificationEvents(t *testing.T) {
	s := testServer(t)
	n := &eventNotifier{}
	s.Webhook = n
	cr := createWithToken(t, s, `{"payload":"`+validPayload+`","ttl":"1h","kind":"text"}`)
	do(t, s, "POST", "/api/drops/"+cr.ID+"/revoke", nil, auth(cr.OwnerToken))
	wait := time.Now().Add(time.Second)
	for time.Now().Before(wait) && len(n.snapshot()) < 2 {
		time.Sleep(5 * time.Millisecond)
	}
	found := map[string]bool{}
	for _, ev := range n.snapshot() {
		found[ev.Event] = true
	}
	if !found["drop.created"] || !found["drop.revoked"] {
		t.Fatalf("events = %v, want created and revoked", found)
	}
}

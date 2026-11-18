package telegram

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nekonaru/cache22/internal/webhook"
)

func TestNewDisabledUnlessBothSet(t *testing.T) {
	if n := New("", "chat"); n != nil {
		t.Fatal("empty token must return nil")
	}
	if n := New("tok", ""); n != nil {
		t.Fatal("empty chat id must return nil")
	}
	if n := New("tok", "chat"); n == nil {
		t.Fatal("both set must return a notifier")
	}
}

func TestSendPostsEvent(t *testing.T) {
	var got []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	n := New("tok:abc", "42")
	n.endpoint = ts.URL
	n.hc = ts.Client()

	ev := webhook.Event{Event: "drop.read", ID: "id123", Kind: "text", ReadsLeft: 0, ReadAt: "2026-11-17T00:00:00Z"}
	if err := n.Send(context.Background(), ev); err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	if err := json.Unmarshal(got, &body); err != nil {
		t.Fatal(err)
	}
	if body["chat_id"] != "42" {
		t.Fatalf("chat_id = %v", body["chat_id"])
	}
	text, _ := body["text"].(string)
	for _, want := range []string{"drop.read", "text", "0 reads left"} {
		if !strings.Contains(text, want) {
			t.Fatalf("text missing %q: %q", want, text)
		}
	}
	// Token never appears anywhere in the body.
	if strings.Contains(string(got), "abc") {
		t.Fatal("token leaked into body")
	}
}

func TestSendRejectsNon2xx(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer ts.Close()
	n := New("tok", "42")
	n.endpoint = ts.URL
	n.hc = ts.Client()
	if err := n.Send(context.Background(), webhook.Event{Event: "drop.read", Kind: "text", ReadsLeft: 0, ReadAt: "2026-11-17T00:00:00Z"}); err == nil {
		t.Fatal("non-2xx must be an error")
	}
}

package webhook

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSetFormatAndSignatureEnabled(t *testing.T) {
	c, err := New("https://hook.example/hook")
	if err != nil {
		t.Fatal(err)
	}
	if c.SignatureEnabled() {
		t.Fatal("fresh client should not sign")
	}
	c.SetSigningSecret("x")
	if !c.SignatureEnabled() {
		t.Fatal("secret should enable signing")
	}
	for _, f := range []string{"form", "slack", "json", "bogus"} {
		c.SetFormat(f)
	}
	if c.format != "json" {
		t.Fatalf("unknown format must fall back to json, got %s", c.format)
	}
}

func TestFormFormat(t *testing.T) {
	var gotCT string
	var gotBody []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCT = r.Header.Get("Content-Type")
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()
	c := testClient(t, ts.URL)
	c.SetFormat("form")
	if err := c.Send(context.Background(), Event{Event: "drop.read", ID: "abc", Kind: "text", ReadsLeft: 0, ReadAt: "2026-11-17T00:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	if gotCT != "application/x-www-form-urlencoded" || !strings.Contains(string(gotBody), "event=drop.read") {
		t.Fatalf("bad form: %s %s", gotCT, gotBody)
	}
}

func TestSlackFormat(t *testing.T) {
	var gotBody []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()
	c := testClient(t, ts.URL)
	c.SetFormat("slack")
	if err := c.Send(context.Background(), Event{Event: "drop.read", ID: "abc", Kind: "text", ReadsLeft: 0, ReadAt: "2026-11-17T00:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(gotBody), "Cache22 drop.read") {
		t.Fatalf("bad slack body: %s", gotBody)
	}
}

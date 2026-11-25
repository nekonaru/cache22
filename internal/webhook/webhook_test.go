package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// testClient builds a client pointed at a local httptest server while the
// SSRF validator still sees a public DNS pool.
func testClient(t *testing.T, serverURL string) *Client {
	t.Helper()
	c, err := New("http://hook.example/hook")
	if err != nil {
		t.Fatal(err)
	}
	c.resolve = func(ctx context.Context, host string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("1.1.1.1")}, nil
	}
	host := strings.TrimPrefix(serverURL, "http://")
	c.hc.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		req = req.Clone(req.Context())
		req.URL.Scheme = "http"
		req.URL.Host = host
		return http.DefaultTransport.RoundTrip(req)
	})
	return c
}

func signature(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// verifySignature is the independent check a receiver would run.
func verifySignature(secret string, body []byte, header string) bool {
	return hmac.Equal([]byte(header), []byte(signature(secret, body)))
}

func TestNewRejectsBadURLs(t *testing.T) {
	for _, raw := range []string{"", "ftp://example.com/x", "https://", "nota url", "http://"} {
		if _, err := New(raw); err == nil {
			t.Fatalf("%q should be rejected", raw)
		}
	}
}

func TestNewStripsCredentials(t *testing.T) {
	c, err := New("https://user:pass@example.com/hook")
	if err != nil {
		t.Fatal(err)
	}
	if c.url.User != nil {
		t.Fatal("credentials must be stripped from the endpoint")
	}
}

func TestIsPrivate(t *testing.T) {
	cases := map[string]bool{
		"127.0.0.1":            true,
		"::1":                  true,
		"10.0.0.1":             true,
		"172.16.0.1":           true,
		"172.31.255.1":         true,
		"192.168.0.1":          true,
		"169.254.1.1":          true,
		"100.64.0.1":           true,
		"100.127.255.254":      true,
		"224.0.0.1":            true,
		"0.0.0.0":              true,
		"fc00::1":              true,
		"fd00::1":              true,
		"fe80::1":              true,
		"8.8.8.8":              false,
		"1.1.1.1":              false,
		"2606:4700:4700::1111": false,
	}
	for raw, want := range cases {
		got := IsPrivate(net.ParseIP(raw))
		if got != want {
			t.Fatalf("IsPrivate(%s) = %v, want %v", raw, got, want)
		}
	}
}

func TestValidateEndpointLiteralIP(t *testing.T) {
	c, err := New("http://127.0.0.1:8080/hook")
	if err != nil {
		t.Fatal(err)
	}
	if err := c.validateEndpoint(context.Background()); err == nil {
		t.Fatal("loopback endpoint must be rejected")
	}
	c2, err := New("http://8.8.8.8/hook")
	if err != nil {
		t.Fatal(err)
	}
	if err := c2.validateEndpoint(context.Background()); err != nil {
		t.Fatalf("public endpoint should pass: %v", err)
	}
}

func TestValidateEndpointDNSResolution(t *testing.T) {
	c, err := New("http://internal.example/hook")
	if err != nil {
		t.Fatal(err)
	}
	c.resolve = func(ctx context.Context, host string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("192.168.1.10")}, nil
	}
	if err := c.validateEndpoint(context.Background()); err == nil {
		t.Fatal("host resolving to private address must be rejected")
	}
	c.resolve = func(ctx context.Context, host string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("192.168.1.10"), net.ParseIP("8.8.8.8")}, nil
	}
	if err := c.validateEndpoint(context.Background()); err == nil {
		t.Fatal("a single private address in the pool must reject the endpoint")
	}
	c.resolve = func(ctx context.Context, host string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("1.1.1.1"), net.ParseIP("8.8.8.8")}, nil
	}
	if err := c.validateEndpoint(context.Background()); err != nil {
		t.Fatalf("public pool should pass: %v", err)
	}
	c.resolve = func(ctx context.Context, host string) ([]net.IP, error) {
		return nil, errors.New("nxdomain")
	}
	if err := c.validateEndpoint(context.Background()); err == nil {
		t.Fatal("DNS failure must be reported")
	}
}

func TestSendPostsEvent(t *testing.T) {
	var got []byte
	var gotCT, gotUA string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, _ = io.ReadAll(r.Body)
		gotCT = r.Header.Get("Content-Type")
		gotUA = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	c := testClient(t, ts.URL)
	ev := Event{Event: "drop.read", ID: "abc", Kind: "text", ReadsLeft: 0, ReadAt: "2026-11-17T00:00:00Z"}
	if err := c.Send(context.Background(), ev); err != nil {
		t.Fatal(err)
	}
	body := string(got)
	for _, want := range []string{`"event":"drop.read"`, `"kind":"text"`, `"reads_left":0`, `"read_at":"2026-11-17T00:00:00Z"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %s: %s", want, body)
		}
	}
	if gotCT != "application/json" || gotUA != "cache22-webhook/1.0" {
		t.Fatalf("headers ct=%s ua=%s", gotCT, gotUA)
	}
}

func TestWebhookSignatureHeader(t *testing.T) {
	var got []byte
	var gotSig string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, _ = io.ReadAll(r.Body)
		gotSig = r.Header.Get("X-Cache22-Signature")
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	c := testClient(t, ts.URL)
	c.SetSigningSecret("audit-secret")
	ev := Event{Event: "drop.read", ID: "abc", Kind: "text", ReadsLeft: 0, ReadAt: "2026-11-17T00:00:00Z"}
	if err := c.Send(context.Background(), ev); err != nil {
		t.Fatal(err)
	}
	if !verifySignature("audit-secret", got, gotSig) {
		t.Fatalf("signature %q does not verify against body %s", gotSig, got)
	}
	if verifySignature("wrong-secret", got, gotSig) {
		t.Fatal("wrong secret must not verify")
	}
}

func TestNoSignatureWithoutSecret(t *testing.T) {
	var gotSig string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSig = r.Header.Get("X-Cache22-Signature")
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	c := testClient(t, ts.URL)
	if err := c.Send(context.Background(), Event{Event: "drop.read", Kind: "text", ReadsLeft: 0, ReadAt: "2026-11-17T00:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	if gotSig != "" {
		t.Fatalf("unexpected signature header %q", gotSig)
	}
}

func TestWebhookSignatureWrongSecret(t *testing.T) {
	body := []byte(`{"event":"drop.read"}`)
	good := signature("right", body)
	bad := signature("wrong", body)
	if good == bad {
		t.Fatal("different secrets must not produce the same signature")
	}
	if !verifySignature("right", body, good) {
		t.Fatal("correct secret must verify")
	}
	if verifySignature("wrong", body, good) {
		t.Fatal("wrong secret must not verify")
	}
}

func TestSendRejectsPrivateEndpoint(t *testing.T) {
	c, err := New("http://192.168.1.1/hook")
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Send(context.Background(), Event{Event: "drop.read"}); err == nil {
		t.Fatal("private endpoint must fail")
	}
}

func TestSendRejectsRedirect(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://example.com/", http.StatusFound)
	}))
	defer ts.Close()
	c := testClient(t, ts.URL)
	if err := c.Send(context.Background(), Event{Event: "drop.read"}); err == nil {
		t.Fatal("redirect must be refused")
	}
}

func TestEventMarshalIsStable(t *testing.T) {
	a, err := json.Marshal(Event{Event: "drop.read", ID: "x", Kind: "text", ReadsLeft: 0, ReadAt: "2026-11-17T00:00:00Z"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(Event{Event: "drop.read", ID: "x", Kind: "text", ReadsLeft: 0, ReadAt: "2026-11-17T00:00:00Z"})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) {
		t.Fatal("event JSON must be deterministic for signature verification")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

package server

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nekonaru/cache22/internal/config"
	"github.com/nekonaru/cache22/internal/store"
)

var testNow = time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)

func testConfig() config.Config {
	return config.Config{
		Addr:          ":0",
		BaseURL:       "http://cache22.test",
		MaxTextBytes:  64 * 1024,
		MaxFileBytes:  2 * 1024 * 1024,
		MaxTotalBytes: 64 << 20,
		RatePerMin:    100000, // effectively unlimited unless a test opts in
		Burst:         100000,
	}
}

func testServer(t *testing.T) *Server {
	t.Helper()
	st := store.NewMemory(0)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return New(testConfig(), st, logger, func() time.Time { return testNow })
}

func do(t *testing.T, s *Server, method, path string, body io.Reader, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, body)
	req.RemoteAddr = "203.0.113.1:4321"
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	return rr
}

func createDrop(t *testing.T, s *Server, body string) *httptest.ResponseRecorder {
	t.Helper()
	return do(t, s, http.MethodPost, "/api/drops", strings.NewReader(body), map[string]string{
		"Content-Type": "application/json",
	})
}

const validPayload = "SGVsbG8tV29ybGQ" // "Hello-World", base64url

func TestCreateThenReadOnce(t *testing.T) {
	s := testServer(t)
	rr := createDrop(t, s, `{"payload":"`+validPayload+`","ttl":"1h","kind":"text"}`)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var cr createResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &cr); err != nil {
		t.Fatal(err)
	}
	if len(cr.ID) != 22 || !strings.HasPrefix(cr.URL, "http://cache22.test/d/") {
		t.Fatalf("bad create response %+v", cr)
	}
	if _, err := base64.RawURLEncoding.DecodeString(cr.ID); err != nil {
		t.Fatal("id is not base64url")
	}

	rd := do(t, s, http.MethodPost, "/api/drops/"+cr.ID+"/read", nil, nil)
	if rd.Code != http.StatusOK {
		t.Fatalf("read status = %d", rd.Code)
	}
	var rrBody readResponse
	if err := json.Unmarshal(rd.Body.Bytes(), &rrBody); err != nil {
		t.Fatal(err)
	}
	if rrBody.Payload != validPayload || rrBody.Kind != "text" {
		t.Fatalf("payload mutated: %+v", rrBody)
	}

	rd2 := do(t, s, http.MethodPost, "/api/drops/"+cr.ID+"/read", nil, nil)
	if rd2.Code != http.StatusNotFound {
		t.Fatal("second read must be 404")
	}
}

func TestReadFailuresAreByteIdentical(t *testing.T) {
	s := testServer(t)
	rr := createDrop(t, s, `{"payload":"`+validPayload+`","ttl":"1h","kind":"text"}`)
	var cr createResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &cr)

	// Consume the drop so the "already read" case is real.
	do(t, s, http.MethodPost, "/api/drops/"+cr.ID+"/read", nil, nil)

	// Reference: the "already read" failure defines the canonical response.
	ref := do(t, s, http.MethodPost, "/api/drops/"+cr.ID+"/read", nil, nil)
	wantBody := append([]byte(nil), ref.Body.Bytes()...)
	wantCT := ref.Header().Get("Content-Type")

	cases := map[string]string{
		"already read":  "/api/drops/" + cr.ID + "/read",
		"never existed": "/api/drops/AAAAAAAAAAAAAAAAAAAAAA/read",
		"malformed id":  "/api/drops/not-an-id/read",
		"empty-ish id":  "/api/drops/%E7%9F%AD/read",
	}
	for name, path := range cases {
		r := do(t, s, http.MethodPost, path, nil, nil)
		if r.Code != http.StatusNotFound {
			t.Fatalf("%s: status = %d, want 404", name, r.Code)
		}
		if !bytes.Equal(r.Body.Bytes(), wantBody) {
			t.Fatalf("%s: body differs: %q vs %q", name, r.Body.String(), wantBody)
		}
		if ct := r.Header().Get("Content-Type"); ct != wantCT {
			t.Fatalf("%s: content-type differs: %q vs %q", name, ct, wantCT)
		}
	}
}

func TestExpiredDropNotReadable(t *testing.T) {
	st := store.NewMemory(0)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	now := testNow
	clock := func() time.Time { return now }
	s := New(testConfig(), st, logger, clock)
	rr := createDrop(t, s, `{"payload":"`+validPayload+`","ttl":"10m","kind":"text"}`)
	var cr createResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &cr)

	now = now.Add(11 * time.Minute) // past TTL, no sleep needed
	rd := do(t, s, http.MethodPost, "/api/drops/"+cr.ID+"/read", nil, nil)
	if rd.Code != http.StatusNotFound {
		t.Fatalf("expired drop read status = %d, want 404", rd.Code)
	}
}

func TestSweepRemovesExpired(t *testing.T) {
	st := store.NewMemory(0)
	s := New(testConfig(), st, slog.New(slog.NewTextHandler(io.Discard, nil)), func() time.Time { return testNow })
	rr := createDrop(t, s, `{"payload":"`+validPayload+`","ttl":"10m","kind":"text"}`)
	var cr createResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &cr)
	_ = cr
	if n := st.Sweep(testNow.Add(11 * time.Minute)); n != 1 {
		t.Fatalf("sweep = %d, want 1", n)
	}
}

func TestCreateValidation(t *testing.T) {
	s := testServer(t)
	cases := []struct {
		name string
		body string
		want int
	}{
		{"bad json", `{`, http.StatusBadRequest},
		{"bad ttl", `{"payload":"` + validPayload + `","ttl":"5s","kind":"text"}`, http.StatusBadRequest},
		{"bad kind", `{"payload":"` + validPayload + `","ttl":"1h","kind":"audio"}`, http.StatusBadRequest},
		{"bad payload b64", `{"payload":"!!!","ttl":"1h","kind":"text"}`, http.StatusBadRequest},
		{"empty payload", `{"payload":"","ttl":"1h","kind":"text"}`, http.StatusBadRequest},
		{"max_reads too big", `{"payload":"` + validPayload + `","ttl":"1h","kind":"text","max_reads":11}`, http.StatusBadRequest},
		{"meta not object", `{"payload":"` + validPayload + `","ttl":"1h","kind":"text","meta":[1,2]}`, http.StatusBadRequest},
		{"unknown field", `{"payload":"` + validPayload + `","ttl":"1h","kind":"text","root":true}`, http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rr := createDrop(t, s, tc.body)
			if rr.Code != tc.want {
				t.Fatalf("status = %d, want %d (body %s)", rr.Code, tc.want, rr.Body.String())
			}
		})
	}
}

func TestCreateTextLimitEnforced(t *testing.T) {
	s := testServer(t) // 64 KiB text cap
	big := make([]byte, 64*1024+1)
	for i := range big {
		big[i] = byte(i)
	}
	payload := base64.RawURLEncoding.EncodeToString(big)
	body := `{"payload":"` + payload + `","ttl":"1h","kind":"text"}`
	rr := createDrop(t, s, body)
	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", rr.Code)
	}
}

func scrubNonce(b []byte) []byte {
	re := regexp.MustCompile(`nonce="[^"]*"`)
	return re.ReplaceAll(b, []byte(`nonce="X"`))
}

func TestRecipientPageAlways200SameBytes(t *testing.T) {
	s := testServer(t)
	var first []byte
	for _, id := range []string{"AAAAAAAAAAAAAAAAAAAAAA", "bbbbbbbbbbbbbbbbbbbbbb", "!!invalid!!"} {
		rr := do(t, s, http.MethodGet, "/d/"+id, nil, nil)
		if rr.Code != http.StatusOK {
			t.Fatalf("GET /d/%s status = %d, want 200", id, rr.Code)
		}
		body := scrubNonce(rr.Body.Bytes())
		if first == nil {
			first = body
			continue
		}
		if !bytes.Equal(first, body) {
			t.Fatalf("page differs for id %q (after nonce scrub)", id)
		}
	}
}

func TestSecurityHeadersPresent(t *testing.T) {
	s := testServer(t)
	rr := do(t, s, http.MethodGet, "/", nil, nil)
	for _, h := range []string{"Cache-Control", "Referrer-Policy", "X-Content-Type-Options", "X-Robots-Tag", "Content-Security-Policy"} {
		if rr.Header().Get(h) == "" {
			t.Fatalf("missing header %s", h)
		}
	}
	if cc := rr.Header().Get("Cache-Control"); cc != "no-store" {
		t.Fatalf("Cache-Control = %q", cc)
	}
	if rp := rr.Header().Get("Referrer-Policy"); rp != "no-referrer" {
		t.Fatalf("Referrer-Policy = %q", rp)
	}
	csp := rr.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "default-src 'none'") || !strings.Contains(csp, "nonce-") {
		t.Fatalf("weak CSP: %s", csp)
	}
}

func TestRateLimited(t *testing.T) {
	cfg := testConfig()
	cfg.RatePerMin = 60 // 1/s
	cfg.Burst = 2       // capacity 2
	st := store.NewMemory(0)
	s := New(cfg, st, slog.New(slog.NewTextHandler(io.Discard, nil)), func() time.Time { return testNow })
	body := `{"payload":"` + validPayload + `","ttl":"1h","kind":"text"}`
	if rr := createDrop(t, s, body); rr.Code != http.StatusCreated {
		t.Fatalf("1st: %d", rr.Code)
	}
	if rr := createDrop(t, s, body); rr.Code != http.StatusCreated {
		t.Fatalf("2nd: %d", rr.Code)
	}
	rr := createDrop(t, s, body)
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("3rd status = %d, want 429", rr.Code)
	}
	if rr.Header().Get("Retry-After") == "" {
		t.Fatal("429 must carry Retry-After")
	}
}

func TestHealthz(t *testing.T) {
	s := testServer(t)
	rr := do(t, s, http.MethodGet, "/healthz", nil, nil)
	if rr.Code != http.StatusOK || rr.Body.String() != "ok" {
		t.Fatalf("healthz = %d %q", rr.Code, rr.Body.String())
	}
}

func TestGzipCompression(t *testing.T) {
	s := testServer(t)
	// Accepts gzip: body must be compressed and carry Content-Encoding.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.1:4321"
	req.Header.Set("Accept-Encoding", "gzip")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if got := rr.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip (page is HTML)", got)
	}
	zr, err := gzip.NewReader(bytes.NewReader(rr.Body.Bytes()))
	if err != nil {
		t.Fatalf("gzip reader: %v", err)
	}
	decompressed, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("gunzip: %v", err)
	}
	if !bytes.Contains(decompressed, []byte("cache")) {
		t.Fatal("decompressed page does not look like the homepage")
	}
	if vary := rr.Header().Get("Vary"); vary != "Accept-Encoding" {
		t.Fatalf("Vary = %q, want Accept-Encoding", vary)
	}

	// Does not accept gzip: identity response, no header.
	rr2 := do(t, s, http.MethodGet, "/", nil, nil)
	if got := rr2.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("identity client got Content-Encoding %q", got)
	}

	// JSON API: gzip applies as well.
	create := func() *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, "/api/drops", strings.NewReader(`{"payload":"`+validPayload+`","ttl":"1h","kind":"text"}`))
		r.RemoteAddr = "203.0.113.1:4321"
		r.Header.Set("Accept-Encoding", "gzip")
		r.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		s.Handler().ServeHTTP(rec, r)
		return rec
	}
	rr3 := create()
	if got := rr3.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("API Content-Encoding = %q, want gzip", got)
	}
}

func TestNewSecurityHeaders(t *testing.T) {
	s := testServer(t)
	rr := do(t, s, http.MethodGet, "/", nil, nil)
	for _, h := range []string{"Cross-Origin-Opener-Policy", "Cross-Origin-Resource-Policy", "Permissions-Policy"} {
		if rr.Header().Get(h) == "" {
			t.Fatalf("missing header %s", h)
		}
	}
	// HSTS must NOT be set on plain HTTP (default config has TrustProxy=false).
	if got := rr.Header().Get("Strict-Transport-Security"); got != "" {
		t.Fatalf("HSTS must be absent without -trust-proxy, got %q", got)
	}
}

func TestConcurrentReadExactlyOneWins(t *testing.T) {
	s := testServer(t)
	rr := createDrop(t, s, `{"payload":"`+validPayload+`","ttl":"1h","kind":"text"}`)
	var cr createResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &cr)

	const n = 100
	var wg sync.WaitGroup
	var mu sync.Mutex
	codes := map[int]int{}
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodPost, "/api/drops/"+cr.ID+"/read", nil)
			req.RemoteAddr = "198.51.100.7:1234"
			rec := httptest.NewRecorder()
			s.Handler().ServeHTTP(rec, req)
			mu.Lock()
			codes[rec.Code]++
			mu.Unlock()
		}()
	}
	wg.Wait()
	if codes[http.StatusOK] != 1 || codes[http.StatusNotFound] != n-1 {
		t.Fatalf("codes = %v, want {200:1, 404:%d}", codes, n-1)
	}
}

func TestNoSensitiveDataInLogs(t *testing.T) {
	var buf bytes.Buffer
	st := store.NewMemory(0)
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	s := New(testConfig(), st, logger, func() time.Time { return testNow })
	rr := createDrop(t, s, `{"payload":"`+validPayload+`","ttl":"1h","kind":"text","meta":{"note":"meta-marker-xyz"}}`)
	var cr createResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &cr)
	if cr.ID == "" {
		t.Fatal("create failed in log-inspection test")
	}
	do(t, s, http.MethodPost, "/api/drops/"+cr.ID+"/read", nil, nil)

	out := buf.String()
	for _, leak := range []string{cr.ID, validPayload, "meta-marker-xyz"} {
		if strings.Contains(out, leak) {
			t.Fatalf("log leaked %q:\n%s", leak, out)
		}
	}
	if !strings.Contains(out, "route=") {
		t.Fatal("log should contain route template")
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

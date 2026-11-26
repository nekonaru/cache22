// Package server implements the Cache22 HTTP API and pages.
//
// Security ground rules (see PRD): GET never consumes a drop; all read
// failures are byte-identical 404s; the server never sees plaintext or key
// material; logs carry method, route template, status, and duration only.
package server

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/nekonaru/cache22/internal/config"
	"github.com/nekonaru/cache22/internal/ratelimit"
	"github.com/nekonaru/cache22/internal/store"
	"github.com/nekonaru/cache22/internal/webhook"
	"github.com/nekonaru/cache22/web"
)

// allowed TTLs, exactly the set exposed by the UI.
var ttlDurations = map[string]time.Duration{
	"10m": 10 * time.Minute,
	"1h":  time.Hour,
	"24h": 24 * time.Hour,
	"7d":  7 * 24 * time.Hour,
}

func (s *Server) allowedTTL() map[string]time.Duration {
	if strings.TrimSpace(s.cfg.TTLAllowlist) == "" {
		return ttlDurations
	}
	out := make(map[string]time.Duration)
	for _, raw := range strings.Split(s.cfg.TTLAllowlist, ",") {
		raw = strings.TrimSpace(raw)
		d, err := config.ParseTTL(raw)
		if err != nil {
			continue
		}
		out[raw] = d
	}
	return out
}

const (
	maxReadsLimit = 10
	idAlphabet    = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	maxMetaBytes  = 2048
)

// Server wires routes, middleware, and dependencies.
type Server struct {
	cfg      config.Config
	store    store.Store
	limits   *ratelimit.Limiter
	log      *slog.Logger
	now      func() time.Time
	page     []byte
	handler  http.Handler
	Webhook  webhook.Notifier // optional; nil means disabled
	Telegram webhook.Notifier // optional; nil means disabled

	created atomic.Int64
	read    atomic.Int64
	revoked atomic.Int64
}

// New builds the handler. now may be nil (defaults to time.Now); tests
// inject a fake clock for expiry scenarios.
func New(cfg config.Config, st store.Store, logger *slog.Logger, now func() time.Time) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	if now == nil {
		now = time.Now
	}
	s := &Server{
		cfg:    cfg,
		store:  st,
		limits: ratelimit.New(cfg.RatePerMin/60, cfg.Burst),
		log:    logger,
		now:    now,
		page:   web.Page,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.createPage)
	mux.HandleFunc("GET /d/{id}", s.recipientPage)
	mux.HandleFunc("POST /api/drops", s.createDrop)
	mux.HandleFunc("POST /api/drops/{id}/read", s.readDrop)
	mux.HandleFunc("GET /api/drops/{id}/status", s.statusDrop)
	mux.HandleFunc("POST /api/drops/{id}/revoke", s.revokeDrop)
	mux.HandleFunc("POST /api/drops/{id}/extend", s.extendDrop)
	mux.HandleFunc("GET /healthz", s.healthz)
	mux.HandleFunc("GET /metrics", s.metrics)
	s.handler = s.withMiddleware(mux)
	return s
}

// Handler exposes the root handler (used by main and by httptest).
func (s *Server) Handler() http.Handler { return s.handler }

// Sweeper periodically evicts expired drops; stop it by cancelling ctx.
func (s *Server) Sweeper(ctxDone <-chan struct{}, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	emit := func(now time.Time) {
		for _, id := range s.store.SweepIDs(now) {
			s.notify(webhook.Event{Event: "drop.expired", ID: id, ReadsLeft: 0, ReadAt: now.UTC().Format(time.RFC3339)})
		}
	}
	for {
		select {
		case <-ctxDone:
			emit(s.now())
			return
		case tick := <-t.C:
			emit(tick)
		}
	}
}

// ---- middleware ----

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// Flush implements http.Flusher so streaming-aware handlers keep working.
func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (s *Server) withMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := s.now()
		nonce, err := newNonce()
		if err != nil {
			// crypto/rand failure: serve a hard failure, never a silent insecure page
			http.Error(w, "internal_error", http.StatusInternalServerError)
			return
		}
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		setSecurityHeaders(sw, nonce, s.cfg.TrustProxy)
		if strings.HasPrefix(r.URL.Path, "/api/") {
			sw.Header().Set("X-RateLimit-Limit", strconv.Itoa(int(s.cfg.RatePerMin)))
			sw.Header().Set("X-RateLimit-Remaining", strconv.Itoa(s.limits.Remaining(s.clientIP(r))))
		}
		if s.cfg.HSTSMaxAge > 0 {
			sw.Header().Set("Strict-Transport-Security", "max-age="+strconv.Itoa(s.cfg.HSTSMaxAge)+"; includeSubDomains")
		}

		// Wrap the response in a gzip writer only when the client accepts
		// gzip; the decision itself is made lazily on the first Write so it
		// can see the Content-Type each handler actually sets.
		var inner http.ResponseWriter = sw
		gzipWriter := &gzipResponseWriter{ResponseWriter: sw, sw: sw, reqAcceptsGzip: strings.Contains(r.Header.Get("Accept-Encoding"), "gzip")}
		inner = gzipWriter
		next.ServeHTTP(&nonceWriter{ResponseWriter: inner, nonce: nonce}, r)
		if gzipWriter.gz != nil {
			_ = gzipWriter.gz.Close()
		}

		path := r.Pattern
		if path == "" {
			path = "unmatched"
		}
		s.log.Info("http",
			"method", r.Method,
			"route", path,
			"status", sw.status,
			"dur", time.Since(start).Round(time.Microsecond),
		)
	})
}

func setSecurityHeaders(w http.ResponseWriter, nonce string, trustProxy bool) {
	h := w.Header()
	h.Set("Cache-Control", "no-store")
	h.Set("Referrer-Policy", "no-referrer")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("X-Robots-Tag", "noindex")
	h.Set("Cross-Origin-Opener-Policy", "same-origin")
	h.Set("Cross-Origin-Resource-Policy", "same-origin")
	h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), interest-cohort=()")
	if trustProxy {
		// HSTS only when the deployment terminates TLS ahead of us.
		h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
	}
	h.Set("Content-Security-Policy", strings.Join([]string{
		"default-src 'none'",
		fmt.Sprintf("script-src 'nonce-%s'", nonce),
		fmt.Sprintf("style-src 'nonce-%s'", nonce),
		"img-src 'self' data:",
		"connect-src 'self'",
		"base-uri 'none'",
		"form-action 'none'",
		"frame-ancestors 'none'",
	}, "; "))
}

// gzipResponseWriter decides lazily, on first write or WriteHeader, whether
// to encode this response with gzip. Eligibility is based on the response
// Content-Type so unknown or binary routes pass through untouched.
type gzipResponseWriter struct {
	http.ResponseWriter
	sw             *statusWriter
	reqAcceptsGzip bool
	gzipOn         bool
	decided        bool
	gz             *gzip.Writer
}

func isCompressibleCT(ct string) bool {
	ct = strings.ToLower(ct)
	for _, frag := range []string{"text/html", "text/plain", "application/json", "image/svg+xml"} {
		if strings.Contains(ct, frag) {
			return true
		}
	}
	return false
}

func (w *gzipResponseWriter) decide() {
	if w.decided {
		return
	}
	w.decided = true
	if w.reqAcceptsGzip {
		w.Header().Add("Vary", "Accept-Encoding")
	}
	if !w.reqAcceptsGzip {
		return
	}
	if isCompressibleCT(w.Header().Get("Content-Type")) {
		w.gzipOn = true
		w.Header().Set("Content-Encoding", "gzip")
	}
}

func (w *gzipResponseWriter) WriteHeader(code int) {
	w.decide()
	if w.gzipOn && w.gz == nil {
		w.gz = gzip.NewWriter(w.ResponseWriter)
	}
	w.sw.WriteHeader(code)
}

func (w *gzipResponseWriter) Write(p []byte) (int, error) {
	w.decide()
	if w.gzipOn && w.gz == nil {
		w.gz = gzip.NewWriter(w.ResponseWriter)
	}
	if w.gz != nil {
		return w.gz.Write(p)
	}
	return w.ResponseWriter.Write(p)
}

func (w *gzipResponseWriter) Flush() {
	if w.gz != nil {
		_ = w.gz.Flush()
	}
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// nonceWriter injects the per-request CSP nonce into embedded HTML pages.
// API/JSON responses pass through untouched.
type nonceWriter struct {
	http.ResponseWriter
	nonce string
}

func (w *nonceWriter) Write(p []byte) (int, error) {
	if strings.Contains(w.Header().Get("Content-Type"), "text/html") {
		p = bytes.ReplaceAll(p, []byte("__CSP_NONCE__"), []byte(w.nonce))
	}
	return w.ResponseWriter.Write(p)
}

// Flush implements http.Flusher so upgrade/streaming paths keep working.
func (w *nonceWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// ---- helpers ----

func newNonce() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func randomID() (string, error) {
	b := make([]byte, 16) // 128 bits
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// newOwnerToken returns a raw 256-bit capability and its hash. Only the
// hash reaches storage; the raw value is returned once to the creator.
func newOwnerToken() (string, []byte, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", nil, err
	}
	h := sha256.Sum256(b)
	return base64.RawURLEncoding.EncodeToString(b), h[:], nil
}

func ownerHashFromRequest(r *http.Request) ([]byte, bool) {
	const prefix = "Bearer "
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, prefix) {
		return nil, false
	}
	raw := strings.TrimSpace(strings.TrimPrefix(h, prefix))
	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil || len(b) != 32 {
		return nil, false
	}
	sum := sha256.Sum256(b)
	return sum[:], true
}

func validID(id string) bool {
	if len(id) != 22 {
		return false
	}
	for _, r := range id {
		if !strings.ContainsRune(idAlphabet, r) {
			return false
		}
	}
	return true
}

func (s *Server) originOK(r *http.Request) bool {
	if !s.cfg.RequireOrigin || r.Method == http.MethodGet || r.Method == http.MethodHead {
		return true
	}
	o := r.Header.Get("Origin")
	if o == "" {
		return true // CLI and curl do not send Origin
	}
	base := strings.TrimSuffix(s.cfg.BaseURL, "/")
	return strings.HasPrefix(o, base)
}

func (s *Server) clientIP(r *http.Request) string {
	if s.cfg.TrustProxy {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			if ip, _, ok := strings.Cut(xff, ","); ok {
				if ip = strings.TrimSpace(ip); ip != "" {
					return ip
				}
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeNotFound is the single uniform failure for every read-path problem:
// unknown id, expired drop, exhausted reads, malformed id - byte-identical.
func writeNotFound(w http.ResponseWriter) {
	writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
}

func writeError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, map[string]string{"error": code})
}

// ---- handlers ----

func (s *Server) metrics(w http.ResponseWriter, r *http.Request) {
	if s.cfg.MetricsToken == "" {
		writeNotFound(w)
		return
	}
	if r.Header.Get("Authorization") != "Bearer "+s.cfg.MetricsToken {
		writeNotFound(w)
		return
	}
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "cache22_drops_created_total %d\n", s.created.Load())
	fmt.Fprintf(w, "cache22_drops_read_total %d\n", s.read.Load())
	fmt.Fprintf(w, "cache22_drops_revoked_total %d\n", s.revoked.Load())
}

func (s *Server) healthz(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("format") == "json" {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "store": "ready"})
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (s *Server) createPage(w http.ResponseWriter, _ *http.Request) {
	s.servePage(w)
}

func (s *Server) recipientPage(w http.ResponseWriter, _ *http.Request) {
	// Deliberately ignores {id}: the page is identical for every id and
	// must not touch the store. Link-preview bots get this page too.
	s.servePage(w)
}

func (s *Server) ttlOptions() []byte {
	var b strings.Builder
	for _, raw := range strings.Split(s.cfg.TTLAllowlist, ",") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		selected := ""
		if raw == "1h" {
			selected = " selected"
		}
		fmt.Fprintf(&b, "<option value=%q%s data-i18n=\"ttl_%s\">%s</option>", raw, selected, raw, raw)
	}
	return []byte(b.String())
}

func (s *Server) servePage(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(bytes.ReplaceAll(s.page, []byte("__TTL_OPTIONS__"), s.ttlOptions()))
}

type createRequest struct {
	Payload  string          `json:"payload"`
	TTL      string          `json:"ttl"`
	MaxReads int             `json:"max_reads"`
	Kind     string          `json:"kind"`
	Meta     json.RawMessage `json:"meta,omitempty"`
}

type createResponse struct {
	ID        string `json:"id"`
	URL       string `json:"url"`
	ExpiresAt string `json:"expires_at"`
	// OwnerToken is a 256-bit capability for revoke, status and one TTL
	// extension. It is returned only here and never stored in plaintext.
	OwnerToken string `json:"owner_token"`
}

func (s *Server) createDrop(w http.ResponseWriter, r *http.Request) {
	if !s.originOK(r) {
		writeNotFound(w)
		return
	}
	if !s.limits.Allow(s.clientIP(r)) {
		w.Header().Set("Retry-After", "60")
		writeError(w, http.StatusTooManyRequests, "rate_limited")
		return
	}

	// Absolute body cap: payload is base64url (~4/3x) of the largest allowed
	// file, plus JSON envelope overhead.
	maxBody := int64(s.cfg.MaxFileBytes)*2 + 8192
	r.Body = http.MaxBytesReader(w, r.Body, maxBody)
	var req createRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeError(w, http.StatusRequestEntityTooLarge, "too_large")
			return
		}
		writeError(w, http.StatusBadRequest, "invalid_request")
		return
	}

	ttl, ok := s.allowedTTL()[req.TTL]
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_ttl")
		return
	}
	if req.Kind != store.KindText && req.Kind != store.KindFile && req.Kind != "files" {
		writeError(w, http.StatusBadRequest, "invalid_kind")
		return
	}
	if req.MaxReads == 0 {
		req.MaxReads = 1
	}
	if req.MaxReads < 1 || req.MaxReads > maxReadsLimit {
		writeError(w, http.StatusBadRequest, "invalid_max_reads")
		return
	}

	// Validate payload: base64url, decoded size capped per kind.
	decoded, err := base64.RawURLEncoding.DecodeString(req.Payload)
	if err != nil || len(req.Payload) == 0 {
		writeError(w, http.StatusBadRequest, "invalid_payload")
		return
	}
	limit := s.cfg.MaxTextBytes
	if req.Kind == store.KindFile {
		limit = s.cfg.MaxFileBytes
	}
	if len(decoded) > limit {
		writeError(w, http.StatusRequestEntityTooLarge, "too_large")
		return
	}

	// Meta: optional, must be a small JSON object, stored opaquely.
	var metaObj map[string]any
	if len(req.Meta) > 0 && !bytes.Equal(bytes.TrimSpace(req.Meta), []byte("null")) {
		if len(req.Meta) > maxMetaBytes {
			writeError(w, http.StatusBadRequest, "invalid_meta")
			return
		}
		if err := json.Unmarshal(req.Meta, &metaObj); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_meta")
			return
		}
	}
	if metaObj == nil {
		metaObj = make(map[string]any)
	}
	// original_reads lets the owner status endpoint report successful
	// recipients without exposing any plaintext or drop contents.
	metaObj["original_reads"] = req.MaxReads
	meta, err := json.Marshal(metaObj)
	if err != nil || len(meta) > maxMetaBytes {
		writeError(w, http.StatusBadRequest, "invalid_meta")
		return
	}

	id, err := randomID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	now := s.now()
	ownerToken, ownerHash, err := newOwnerToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	d := store.NewDrop(id, req.Payload, req.Kind, meta, req.MaxReads, now, ttl, len(decoded))
	d.OwnerHash = ownerHash
	if err := s.store.Put(d); err != nil {
		if errors.Is(err, store.ErrFull) {
			writeError(w, http.StatusServiceUnavailable, "capacity_reached")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	s.created.Add(1)
	s.notify(webhook.Event{Event: "drop.created", ID: id, Kind: d.Kind, ReadsLeft: d.ReadsLeft, ReadAt: now.UTC().Format(time.RFC3339)})
	writeJSON(w, http.StatusCreated, createResponse{
		ID:         id,
		URL:        s.cfg.BaseURL + "/d/" + id,
		ExpiresAt:  now.Add(ttl).UTC().Format(time.RFC3339),
		OwnerToken: ownerToken,
	})
}

type readResponse struct {
	Payload string          `json:"payload"`
	Kind    string          `json:"kind"`
	Meta    json.RawMessage `json:"meta,omitempty"`
}

type statusResponse struct {
	Status     string `json:"status"`
	Kind       string `json:"kind"`
	ReadsLeft  int    `json:"reads_left"`
	ExpiresAt  string `json:"expires_at"`
	CanExtend  bool   `json:"can_extend"`
	Recipients int    `json:"recipients"` // number of successful reads so far
}

type extendRequest struct {
	TTL string `json:"ttl"`
}

// notify fans a read receipt out to every configured notifier without
// blocking the request path. Failures are logged without ids or secrets.
func (s *Server) notify(ev webhook.Event) {
	if s.Webhook == nil && s.Telegram == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		defer cancel()
		if s.Webhook != nil {
			if err := s.Webhook.Send(ctx, ev); err != nil {
				s.log.Warn("webhook: send failed", "err", err)
			}
		}
		if s.Telegram != nil {
			if err := s.Telegram.Send(ctx, ev); err != nil {
				s.log.Warn("telegram: send failed", "err", err)
			}
		}
	}()
}

func (s *Server) statusDrop(w http.ResponseWriter, r *http.Request) {
	if !s.originOK(r) {
		writeNotFound(w)
		return
	}
	if !s.limits.Allow(s.clientIP(r)) {
		w.Header().Set("Retry-After", "60")
		writeError(w, http.StatusTooManyRequests, "rate_limited")
		return
	}
	id := r.PathValue("id")
	hash, ok := ownerHashFromRequest(r)
	if !ok || !validID(id) {
		writeNotFound(w)
		return
	}
	now := s.now()
	d, ok := s.store.OwnerGet(id, hash, now)
	if !ok {
		writeNotFound(w)
		return
	}
	recipients := 0 // derived from the original read count when available
	if d.Meta != nil {
		var m struct {
			OriginalReads int `json:"original_reads"`
		}
		_ = json.Unmarshal(d.Meta, &m)
		if m.OriginalReads > d.ReadsLeft {
			recipients = m.OriginalReads - d.ReadsLeft
		}
	}
	writeJSON(w, http.StatusOK, statusResponse{
		Status: "live", Kind: d.Kind, ReadsLeft: d.ReadsLeft,
		ExpiresAt: d.ExpiresAt.UTC().Format(time.RFC3339),
		CanExtend: !d.Extended, Recipients: recipients,
	})
}

func (s *Server) revokeDrop(w http.ResponseWriter, r *http.Request) {
	if !s.originOK(r) {
		writeNotFound(w)
		return
	}
	if !s.limits.Allow(s.clientIP(r)) {
		w.Header().Set("Retry-After", "60")
		writeError(w, http.StatusTooManyRequests, "rate_limited")
		return
	}
	id := r.PathValue("id")
	hash, ok := ownerHashFromRequest(r)
	if !ok || !validID(id) {
		writeNotFound(w)
		return
	}
	if !s.store.OwnerDelete(id, hash, s.now()) {
		writeNotFound(w)
		return
	}
	s.revoked.Add(1)
	s.notify(webhook.Event{Event: "drop.revoked", ID: id, Kind: "text", ReadsLeft: 0, ReadAt: s.now().UTC().Format(time.RFC3339)})
	writeJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
}

func (s *Server) extendDrop(w http.ResponseWriter, r *http.Request) {
	if !s.originOK(r) {
		writeNotFound(w)
		return
	}
	if !s.limits.Allow(s.clientIP(r)) {
		w.Header().Set("Retry-After", "60")
		writeError(w, http.StatusTooManyRequests, "rate_limited")
		return
	}
	id := r.PathValue("id")
	hash, ok := ownerHashFromRequest(r)
	if !ok || !validID(id) {
		writeNotFound(w)
		return
	}
	now := s.now()
	d, ok := s.store.OwnerGet(id, hash, now)
	if !ok || d.Extended {
		writeNotFound(w)
		return
	}
	var req extendRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if dec.Decode(&req) != nil {
		writeNotFound(w)
		return
	}
	ttl, ok := s.allowedTTL()[req.TTL]
	if !ok {
		writeNotFound(w)
		return
	}
	newExpiry := now.Add(ttl)
	if newExpiry.Before(d.ExpiresAt) {
		writeNotFound(w)
		return
	}
	d.ExpiresAt = newExpiry
	d.Extended = true
	if !s.store.OwnerUpdate(d, hash, now) {
		writeNotFound(w)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "extended", "expires_at": newExpiry.UTC().Format(time.RFC3339),
	})
}

func (s *Server) readDrop(w http.ResponseWriter, r *http.Request) {
	if !s.originOK(r) {
		writeNotFound(w)
		return
	}
	if !s.limits.Allow(s.clientIP(r)) {
		w.Header().Set("Retry-After", "60")
		writeError(w, http.StatusTooManyRequests, "rate_limited")
		return
	}
	id := r.PathValue("id")
	if !validID(id) {
		writeNotFound(w)
		return
	}
	d, ok := s.store.Consume(id, s.now())
	if !ok {
		writeNotFound(w)
		return
	}
	s.notify(webhook.Event{
		Event: "drop.read", ID: d.ID, Kind: d.Kind,
		ReadsLeft: d.ReadsLeft, ReadAt: s.now().UTC().Format(time.RFC3339),
	})
	s.read.Add(1)
	writeJSON(w, http.StatusOK, readResponse{
		Payload: d.Payload,
		Kind:    d.Kind,
		Meta:    d.Meta,
	})
}

// Package server implements the Cache22 HTTP API and pages.
//
// Security ground rules (see PRD): GET never consumes a drop; all read
// failures are byte-identical 404s; the server never sees plaintext or key
// material; logs carry method, route template, status, and duration only.
package server

import (
	"bytes"
	"compress/gzip"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/nekonaru/cache22/internal/config"
	"github.com/nekonaru/cache22/internal/ratelimit"
	"github.com/nekonaru/cache22/internal/store"
	"github.com/nekonaru/cache22/web"
)

// allowed TTLs, exactly the set exposed by the UI.
var ttlDurations = map[string]time.Duration{
	"10m": 10 * time.Minute,
	"1h":  time.Hour,
	"24h": 24 * time.Hour,
	"7d":  7 * 24 * time.Hour,
}

const (
	maxReadsLimit = 10
	idAlphabet    = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	maxMetaBytes  = 2048
)

// Server wires routes, middleware, and dependencies.
type Server struct {
	cfg     config.Config
	store   store.Store
	limits  *ratelimit.Limiter
	log     *slog.Logger
	now     func() time.Time
	page    []byte
	handler http.Handler
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
	mux.HandleFunc("GET /healthz", s.healthz)
	s.handler = s.withMiddleware(mux)
	return s
}

// Handler exposes the root handler (used by main and by httptest).
func (s *Server) Handler() http.Handler { return s.handler }

// Sweeper periodically evicts expired drops; stop it by cancelling ctx.
func (s *Server) Sweeper(ctxDone <-chan struct{}, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctxDone:
			s.store.Sweep(s.now())
			return
		case tick := <-t.C:
			s.store.Sweep(tick)
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
		// HSTS only when the deployment terminates TLS ahead of us (the
		// standard, documented way to run this app). Never over plain HTTP.
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

func (s *Server) healthz(w http.ResponseWriter, _ *http.Request) {
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

func (s *Server) servePage(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(s.page)
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
}

func (s *Server) createDrop(w http.ResponseWriter, r *http.Request) {
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

	ttl, ok := ttlDurations[req.TTL]
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_ttl")
		return
	}
	if req.Kind != store.KindText && req.Kind != store.KindFile {
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
	var meta []byte
	if len(req.Meta) > 0 && !bytes.Equal(bytes.TrimSpace(req.Meta), []byte("null")) {
		if len(req.Meta) > maxMetaBytes {
			writeError(w, http.StatusBadRequest, "invalid_meta")
			return
		}
		var probe map[string]any
		if err := json.Unmarshal(req.Meta, &probe); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_meta")
			return
		}
		meta = append([]byte(nil), req.Meta...)
	}

	id, err := randomID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	now := s.now()
	d := store.NewDrop(id, req.Payload, req.Kind, meta, req.MaxReads, now, ttl, len(decoded))
	if err := s.store.Put(d); err != nil {
		if errors.Is(err, store.ErrFull) {
			writeError(w, http.StatusServiceUnavailable, "capacity_reached")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	writeJSON(w, http.StatusCreated, createResponse{
		ID:        id,
		URL:       s.cfg.BaseURL + "/d/" + id,
		ExpiresAt: now.Add(ttl).UTC().Format(time.RFC3339),
	})
}

type readResponse struct {
	Payload string          `json:"payload"`
	Kind    string          `json:"kind"`
	Meta    json.RawMessage `json:"meta,omitempty"`
}

func (s *Server) readDrop(w http.ResponseWriter, r *http.Request) {
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
	writeJSON(w, http.StatusOK, readResponse{
		Payload: d.Payload,
		Kind:    d.Kind,
		Meta:    d.Meta,
	})
}

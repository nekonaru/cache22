// Package webhook delivers read receipts to an operator-configured URL.
//
// It exists so a sender can learn a drop was opened (P2 in the PRD). The
// payload carries only metadata: no payload, ciphertext, or key fragment.
// The URL is validated against SSRF: only http/https on public addresses
// is permitted, and redirects are refused.
package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"
)

// Event is the notification sent on each consumed read.
type Event struct {
	Event     string `json:"event"` // always "drop.read"
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	ReadsLeft int    `json:"reads_left"`
	ReadAt    string `json:"read_at"` // RFC3339
}

// Notifier sends read receipts. The webhook client satisfies it; the server
// keeps an interface so it can be faked in tests.
type Notifier interface {
	Send(ctx context.Context, ev Event) error
}

// Client sends events. Zero dependencies, safe to reuse.
type Client struct {
	url     *url.URL
	hc      *http.Client
	secret  string // optional HMAC signing key for receiver verification
	resolve func(ctx context.Context, host string) ([]net.IP, error)
}

// New parses and validates the endpoint. Hosts whose DNS resolves to a
// private, loopback, link-local, or otherwise non-public address are
// rejected, as are non-http(s) schemes.
func New(raw string) (*Client, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("webhook: parse url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("webhook: scheme %q must be http or https", u.Scheme)
	}
	if u.Host == "" {
		return nil, errors.New("webhook: url has no host")
	}
	// Strip credentials if any: never send them over a webhook.
	u.User = nil
	return &Client{
		url: u,
		hc: &http.Client{
			Timeout: 5 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		resolve: func(ctx context.Context, host string) ([]net.IP, error) {
			return net.DefaultResolver.LookupIP(ctx, "ip", host)
		},
	}, nil
}

// SetSigningSecret enables an HMAC-SHA256 signature header on each POST.
// It must be called before Send; the key is never transmitted except as
// the signature itself. An empty secret leaves signing off.
func (c *Client) SetSigningSecret(secret string) {
	c.secret = secret
}

// signBody returns a stable HMAC-SHA256 signature over exact raw JSON bytes.
func signBody(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// IsPrivate reports whether ip is private, loopback, link-local, or otherwise
// not a public global address.
func IsPrivate(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}
	if ip.IsMulticast() {
		return true
	}
	// IPv4 CGNAT range 100.64.0.0/10 (RFC 6598).
	if ip4 := ip.To4(); ip4 != nil && ip4[0] == 100 && ip4[1] >= 64 && ip4[1] < 128 {
		return true
	}
	if v4 := ip.To4(); v4 != nil {
		return v4[0] == 10 || v4[0] == 172 && v4[1] >= 16 && v4[1] <= 31 || v4[0] == 192 && v4[1] == 168
	}
	// IPv6 ULA fc00::/7
	if len(ip) == net.IPv6len && (ip[0]&0xfe) == 0xfc {
		return true
	}
	return false
}

// validateEndpoint fails closed if the host is not public right now.
func (c *Client) validateEndpoint(ctx context.Context) error {
	host := c.url.Hostname()
	if ip := net.ParseIP(host); ip != nil {
		if IsPrivate(ip) {
			return fmt.Errorf("webhook: host %s is not a public address", host)
		}
		return nil
	}
	ips, err := c.resolve(ctx, host)
	if err != nil {
		return fmt.Errorf("webhook: resolve %s: %w", host, err)
	}
	if len(ips) == 0 {
		return fmt.Errorf("webhook: resolve %s: no addresses", host)
	}
	for _, ip := range ips {
		if IsPrivate(ip) {
			return fmt.Errorf("webhook: host %s resolves to non-public address %s", host, ip)
		}
	}
	return nil
}

// Send posts the event. Redirects are refused. Failures are returned for
// logging; callers should treat this as best-effort, never as a gate.
func (c *Client) Send(ctx context.Context, ev Event) error {
	if err := c.validateEndpoint(ctx); err != nil {
		return err
	}
	body, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url.String(), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "cache22-webhook/1.0")
	if c.secret != "" {
		req.Header.Set("X-Cache22-Signature", signBody(c.secret, body))
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return fmt.Errorf("webhook: post: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("webhook: status %d", resp.StatusCode)
	}
	return nil
}

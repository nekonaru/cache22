// Package telegram delivers read receipts to a Telegram bot.
//
// It exists so the operator gets a push notification the moment a secret is
// opened (P2 in the PRD). The message carries only metadata: no payload,
// ciphertext, key fragment, or id. The endpoint is pinned to the public
// Telegram Bot API host, and any redirect is refused.
//
// Configure with CACHE22_TELEGRAM_BOT_TOKEN and CACHE22_TELEGRAM_CHAT_ID;
// the notifier stays disabled unless both are set.
package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/nekonaru/cache22/internal/webhook"
)

const defaultEndpoint = "https://api.telegram.org"

// Notifier posts read receipts to Telegram via the Bot API.
type Notifier struct {
	token    string
	chatID   string
	endpoint string
	hc       *http.Client
}

// New validates the token and chat id. An empty token or chat id yields nil
// (disabled), matching "unset means off".
func New(token, chatID string) *Notifier {
	if strings.TrimSpace(token) == "" || strings.TrimSpace(chatID) == "" {
		return nil
	}
	return &Notifier{
		token:    token,
		chatID:   chatID,
		endpoint: defaultEndpoint,
		hc: &http.Client{
			Timeout: 5 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

// Send implements webhook.Notifier so the server can hand the notifier
// the same event the webhook sender consumes.
func (n *Notifier) Send(ctx context.Context, ev webhook.Event) error {
	text := fmt.Sprintf("Cache22: %s (kind %s, %d reads left, %s)",
		ev.Event, ev.Kind, ev.ReadsLeft, ev.ReadAt)
	body, err := json.Marshal(map[string]any{
		"chat_id": n.chatID,
		"text":    text,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("%s/bot%s/sendMessage", n.endpoint, n.token), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "cache22-telegram/1.0")
	req.URL.RawQuery = "" // token rides in the path, never in the query

	resp, err := n.hc.Do(req)
	if err != nil {
		return fmt.Errorf("telegram: post: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("telegram: status %d", resp.StatusCode)
	}
	return nil
}

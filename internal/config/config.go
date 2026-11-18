// Package config resolves runtime configuration from flags and environment
// variables. Environment variables are the fallback for every flag so the
// binary can run twelve-factor style in containers.
package config

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds all knobs for the server.
type Config struct {
	Addr           string // listen address, e.g. ":8080"
	BaseURL        string // public base URL used in created links, e.g. "https://cache22.example.com"
	MaxTextBytes   int    // max decoded payload size for kind=text
	MaxFileBytes   int    // max decoded payload size for kind=file
	MaxTotalBytes  int64  // global storage budget in bytes (applies to both backends)
	RatePerMin     float64
	Burst          float64
	TrustProxy     bool   // trust X-Forwarded-For for client IP (enable only behind a trusted proxy)
	Data           string // bbolt file path; empty means pure in-memory storage
	WebhookURL     string // read receipt endpoint; empty means disabled (env CACHE22_WEBHOOK_URL)
	TelegramToken  string // Telegram bot token; empty disables Telegram read receipts (env CACHE22_TELEGRAM_BOT_TOKEN)
	TelegramChatID string // Telegram chat id for read receipts (env CACHE22_TELEGRAM_CHAT_ID)
}

func envStr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func envInt64(key string, def int64) int64 {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
	}
	return def
}

func envFloat(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.ParseFloat(v, 64); err == nil {
			return n
		}
	}
	return def
}

func envBool(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}

// Parse wires flags to defaults pulled from the environment. It reads from
// the passed FlagSet so tests can build isolated parsers.
func Parse(fs *flag.FlagSet, args []string) (Config, error) {
	var c Config
	fs.StringVar(&c.Addr, "addr", envStr("CACHE22_ADDR", ":8080"), "listen address (env CACHE22_ADDR)")
	fs.StringVar(&c.BaseURL, "base-url", envStr("CACHE22_BASE_URL", "http://localhost:8080"), "public base URL for created links (env CACHE22_BASE_URL)")
	fs.IntVar(&c.MaxTextBytes, "max-text-bytes", envInt("CACHE22_MAX_TEXT_BYTES", 64*1024), "max decoded payload bytes for kind=text (env CACHE22_MAX_TEXT_BYTES)")
	fs.IntVar(&c.MaxFileBytes, "max-file-bytes", envInt("CACHE22_MAX_FILE_BYTES", 2*1024*1024), "max decoded payload bytes for kind=file (env CACHE22_MAX_FILE_BYTES)")
	fs.Int64Var(&c.MaxTotalBytes, "max-total-bytes", envInt64("CACHE22_MAX_TOTAL_BYTES", 64<<20), "global memory budget in bytes for in-memory storage (env CACHE22_MAX_TOTAL_BYTES)")
	fs.Float64Var(&c.RatePerMin, "rate-per-min", envFloat("CACHE22_RATE_PER_MIN", 30), "per-IP sustained request rate per minute (env CACHE22_RATE_PER_MIN)")
	fs.Float64Var(&c.Burst, "burst", envFloat("CACHE22_BURST", 10), "per-IP burst allowance (env CACHE22_BURST)")
	fs.BoolVar(&c.TrustProxy, "trust-proxy", envBool("CACHE22_TRUST_PROXY", false), "trust X-Forwarded-For for client IP; enable only behind a trusted proxy (env CACHE22_TRUST_PROXY)")
	fs.StringVar(&c.Data, "data", envStr("CACHE22_DATA", "cache22.db"), "bbolt data file path (e.g. /var/lib/cache22/cache22.db); set to \"-\" for in-memory storage (env CACHE22_DATA)")
	fs.StringVar(&c.WebhookURL, "webhook", envStr("CACHE22_WEBHOOK_URL", ""), "read receipt webhook URL (e.g. https://hooks.example.com/cache22); empty disables notifications (env CACHE22_WEBHOOK_URL)")
	fs.StringVar(&c.TelegramToken, "telegram-bot-token", envStr("CACHE22_TELEGRAM_BOT_TOKEN", ""), "Telegram bot token for read receipts (env CACHE22_TELEGRAM_BOT_TOKEN)")
	fs.StringVar(&c.TelegramChatID, "telegram-chat-id", envStr("CACHE22_TELEGRAM_CHAT_ID", ""), "Telegram chat id for read receipts (env CACHE22_TELEGRAM_CHAT_ID)")
	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}
	if !strings.HasPrefix(c.BaseURL, "http://") && !strings.HasPrefix(c.BaseURL, "https://") {
		return Config{}, fmt.Errorf("base-url must start with http:// or https://, got %q", c.BaseURL)
	}
	c.BaseURL = strings.TrimSuffix(c.BaseURL, "/")
	if c.MaxTextBytes <= 0 || c.MaxFileBytes <= 0 {
		return Config{}, fmt.Errorf("max-text-bytes and max-file-bytes must be positive")
	}
	return c, nil
}

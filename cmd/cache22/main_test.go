package main

import (
	"log/slog"
	"testing"

	"github.com/nekonaru/cache22/internal/config"
)

func TestWebhookSecretWiring(t *testing.T) {
	cfg := config.Config{
		WebhookURL:   "https://hook.example/cache22",
		NotifySecret: "shared-secret",
	}
	hook, err := makeWebhook(cfg, slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	if hook == nil || !hook.SignatureEnabled() {
		t.Fatal("notify secret must enable webhook signatures")
	}

	cfg.NotifySecret = ""
	hook, err = makeWebhook(cfg, slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	if hook == nil || hook.SignatureEnabled() {
		t.Fatal("empty notify secret must leave signatures off")
	}
}

func TestNoWebhookWithoutURL(t *testing.T) {
	hook, err := makeWebhook(config.Config{}, slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	if hook != nil {
		t.Fatal("empty webhook url must disable the client")
	}
}

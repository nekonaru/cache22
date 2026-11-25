package config

import (
	"bytes"
	"flag"
	"io"
	"strings"
	"testing"
)

func parse(t *testing.T, env map[string]string, args []string) (Config, error) {
	t.Helper()
	for k, v := range env {
		t.Setenv(k, v)
	}
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	return Parse(fs, args)
}

func TestDefaults(t *testing.T) {
	env := map[string]string{}
	if _, err := parse(t, env, nil); err != nil {
		t.Fatal(err)
	}
	// Defaults are applied from Parse itself, not from the environment.
	fs := flag.NewFlagSet("defaults", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	c, err := Parse(fs, nil)
	if err != nil {
		t.Fatal(err)
	}
	if c.Addr != ":8080" || c.BaseURL != "http://localhost:8080" || c.Data != "cache22.db" {
		t.Fatalf("unexpected defaults: %+v", c)
	}
	if c.MaxTextBytes != 65536 || c.MaxFileBytes != 2*1024*1024 || c.MaxTotalBytes != 64<<20 {
		t.Fatalf("unexpected size defaults: %+v", c)
	}
	if c.RatePerMin != 30 || c.Burst != 10 || c.TrustProxy {
		t.Fatalf("unexpected rate defaults: %+v", c)
	}
}

func TestEnvOverrides(t *testing.T) {
	env := map[string]string{
		"CACHE22_ADDR":               ":9999",
		"CACHE22_BASE_URL":           "https://secret.example.com/",
		"CACHE22_MAX_TEXT_BYTES":     "1000",
		"CACHE22_MAX_FILE_BYTES":     "2000",
		"CACHE22_MAX_TOTAL_BYTES":    "4096",
		"CACHE22_RATE_PER_MIN":       "5.5",
		"CACHE22_BURST":              "7",
		"CACHE22_TRUST_PROXY":        "true",
		"CACHE22_DATA":               "-",
		"CACHE22_WEBHOOK_URL":        "https://hook.example/hook",
		"CACHE22_TELEGRAM_BOT_TOKEN": "tok:xyz",
		"CACHE22_TELEGRAM_CHAT_ID":   "42",
		"CACHE22_NOTIFY_SECRET":      "hmac-key",
	}
	c, err := parse(t, env, nil)
	if err != nil {
		t.Fatal(err)
	}
	if c.Addr != ":9999" || c.BaseURL != "https://secret.example.com" {
		t.Fatalf("bad addr/base: %+v", c)
	}
	if c.MaxTextBytes != 1000 || c.MaxFileBytes != 2000 || c.MaxTotalBytes != 4096 {
		t.Fatalf("bad sizes: %+v", c)
	}
	if c.RatePerMin != 5.5 || c.Burst != 7 || !c.TrustProxy {
		t.Fatalf("bad rate/proxy: %+v", c)
	}
	if c.Data != "-" || c.WebhookURL != "https://hook.example/hook" {
		t.Fatalf("bad storage/webhook: %+v", c)
	}
	if c.TelegramToken != "tok:xyz" || c.TelegramChatID != "42" || c.NotifySecret != "hmac-key" {
		t.Fatalf("bad notify config: %+v", c)
	}
}

func TestFlagOverridesEnv(t *testing.T) {
	c, err := parse(t, map[string]string{"CACHE22_ADDR": ":1111"}, []string{"-addr", ":2222"})
	if err != nil {
		t.Fatal(err)
	}
	if c.Addr != ":2222" {
		t.Fatalf("flag must win: %s", c.Addr)
	}
}

func TestInvalidConfig(t *testing.T) {
	keys := []string{
		"CACHE22_BASE_URL", "CACHE22_MAX_TEXT_BYTES", "CACHE22_MAX_FILE_BYTES",
	}
	cases := []struct {
		env  map[string]string
		args []string
		want string
	}{
		{map[string]string{"CACHE22_BASE_URL": "ftp://x"}, nil, "base-url"},
		{map[string]string{"CACHE22_MAX_TEXT_BYTES": "0"}, nil, "max-text-bytes"},
		{map[string]string{"CACHE22_MAX_FILE_BYTES": "-1"}, nil, "max-file-bytes"},
		{nil, []string{"-unknown"}, "flag provided but not defined"},
	}
	for _, tc := range cases {
		for _, k := range keys {
			t.Setenv(k, "")
		}
		_, err := parse(t, tc.env, tc.args)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("case %v: err = %v, want %q", tc, err, tc.want)
		}
	}
}

func TestInvalidEnvFallsBackToDefault(t *testing.T) {
	c, err := parse(t, map[string]string{
		"CACHE22_MAX_TEXT_BYTES": "nope",
		"CACHE22_BURST":          "x",
		"CACHE22_TRUST_PROXY":    "maybe",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if c.MaxTextBytes != 65536 || c.Burst != 10 || c.TrustProxy {
		t.Fatalf("invalid env must fall back: %+v", c)
	}
}

func TestSecretsNeverAppearInFlagOutput(t *testing.T) {
	const secretValue = "SENTINEL_SECRET_VALUE"
	t.Setenv("CACHE22_TELEGRAM_BOT_TOKEN", secretValue)
	t.Setenv("CACHE22_NOTIFY_SECRET", secretValue)

	var buf bytes.Buffer
	fs := flag.NewFlagSet("secret-test", flag.ContinueOnError)
	fs.SetOutput(&buf)
	c, err := Parse(fs, nil)
	if err != nil {
		t.Fatal(err)
	}
	if c.TelegramToken != secretValue || c.NotifySecret != secretValue {
		t.Fatal("environment secrets must still be loaded")
	}
	if strings.Contains(buf.String(), secretValue) {
		t.Fatalf("flag output leaked the secret: %s", buf.String())
	}

	fs2 := flag.NewFlagSet("secret-test-2", flag.ContinueOnError)
	var buf2 bytes.Buffer
	fs2.SetOutput(&buf2)
	if _, err := Parse(fs2, []string{"-bogus"}); err == nil {
		t.Fatal("unknown flag must fail")
	}
	if strings.Contains(buf2.String(), secretValue) {
		t.Fatalf("error output leaked the secret: %s", buf2.String())
	}
}

func TestExplicitSecretFlagWins(t *testing.T) {
	t.Setenv("CACHE22_NOTIFY_SECRET", "from-env")
	fs := flag.NewFlagSet("secret-flag", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	c, err := Parse(fs, []string{"-notify-secret", "from-flag"})
	if err != nil {
		t.Fatal(err)
	}
	if c.NotifySecret != "from-flag" {
		t.Fatalf("explicit flag must win: %q", c.NotifySecret)
	}
}

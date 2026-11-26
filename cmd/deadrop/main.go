// Command deadrop is the Cache22 CLI.
//
//	echo "rahasia" | deadrop            # create a drop, print the link
//	deadrop -file secrets.zip          # create from a file
//	deadrop get https://host/d/ID#KEY # read (and burn) a drop
//
// The server only ever receives ciphertext. The key never leaves this
// process except inside the printed link's URL fragment (or stays wrapped
// under a passphrase in server-side meta).
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nekonaru/cache22/internal/crypt"
)

const pbkdf2Iterations = 600_000

var version = "dev" // stamped via -ldflags "-X main.version=…"

type cliConfig struct {
	server     string
	ttl        string
	maxReads   int
	files      []string
	passphrase string
	out        string
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "deadrop:", err)
		os.Exit(1)
	}
}

func run() error {
	args := os.Args[1:]
	if len(args) > 0 && args[0] == "get" {
		return runGet(args[1:])
	}
	if len(args) > 0 && (args[0] == "help" || args[0] == "-h" || args[0] == "--help") {
		usage()
		return nil
	}
	if len(args) > 0 && (args[0] == "--version" || args[0] == "-version" || args[0] == "version") {
		fmt.Printf("deadrop %s\n", version)
		return nil
	}
	return runCreate(args)
}

func usage() {
	fmt.Fprintf(os.Stderr, `deadrop - Cache22 CLI (one-time secrets)

Usage:
  echo "secret" | deadrop [flags]     create a drop, print the link
  deadrop -file path [flags]         create from a file
  deadrop get <url> [-o file]        read (burn) a drop

Flags (create):
  -server URL        server base URL (env CACHE22_SERVER, default http://localhost:8080)
  -ttl 10m|1h|24h|7d expiry (default 1h)
  -max-reads N       max opens, 1-10 (default 1)
  -file PATH         read the secret from a file instead of stdin
  -passphrase PASS   wrap the key with a passphrase (env CACHE22_PASSPHRASE recommended)

The link's fragment (#…) is the decryption key. It is never sent to the server.
`)
}

// nextFlag returns the value following a flag at position i, never panicking.
func nextFlag(args []string, i int, name string) (string, error) {
	if i+1 >= len(args) {
		return "", fmt.Errorf("%s needs a value", name)
	}
	return args[i+1], nil
}

func parseCreateFlags(args []string) (cliConfig, error) {
	var c cliConfig
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-server":
			v, err := nextFlag(args, i, "-server")
			if err != nil {
				return c, err
			}
			i++
			c.server = v
		case "-ttl":
			v, err := nextFlag(args, i, "-ttl")
			if err != nil {
				return c, err
			}
			i++
			c.ttl = v
		case "-max-reads":
			v, err := nextFlag(args, i, "-max-reads")
			if err != nil {
				return c, err
			}
			i++
			var n int
			if _, err := fmt.Sscanf(v, "%d", &n); err != nil {
				return c, fmt.Errorf("-max-reads: %w", err)
			}
			c.maxReads = n
		case "-file":
			v, err := nextFlag(args, i, "-file")
			if err != nil {
				return c, err
			}
			i++
			c.files = append(c.files, v)
		case "-passphrase":
			v, err := nextFlag(args, i, "-passphrase")
			if err != nil {
				return c, err
			}
			i++
			c.passphrase = v
		default:
			return c, fmt.Errorf("unknown flag %q", args[i])
		}
	}
	if c.server == "" {
		c.server = envOr("CACHE22_SERVER", "http://localhost:8080")
	}
	if c.ttl == "" {
		c.ttl = "1h"
	}
	if c.maxReads == 0 {
		c.maxReads = 1
	}
	if c.maxReads < 1 || c.maxReads > 10 {
		return c, fmt.Errorf("-max-reads must be 1-10")
	}
	switch c.ttl {
	case "10m", "1h", "24h", "7d":
	default:
		return c, fmt.Errorf("-ttl must be one of 10m, 1h, 24h, 7d")
	}
	if c.passphrase == "" {
		c.passphrase = os.Getenv("CACHE22_PASSPHRASE")
	}
	return c, nil
}

func runCreate(args []string) error {
	cfg, err := parseCreateFlags(args)
	if err != nil {
		return err
	}

	var plaintext []byte
	var files []crypt.FileEntry
	kind, name := "text", ""
	total := 0
	for _, path := range cfg.files {
		b, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		total += len(b)
		if total > 2<<20 {
			return errors.New("total file size exceeds 2 MB limit")
		}
		files = append(files, crypt.FileEntry{Name: baseName(path), Data: b})
	}
	if len(files) == 0 {
		b, err := io.ReadAll(io.LimitReader(os.Stdin, 64<<10+1))
		if err != nil {
			return err
		}
		if len(b) > 64<<10 {
			return errors.New("stdin exceeds 64 KB limit (use -file for files)")
		}
		plaintext = b
	} else if len(files) == 1 {
		kind, name = "file", files[0].Name
		plaintext = files[0].Data
	} else {
		kind = "files"
	}
	if len(plaintext) == 0 && len(files) == 0 {
		return errors.New("nothing to send (empty input)")
	}

	key, err := crypt.GenerateKey()
	if err != nil {
		return err
	}
	var payload string
	if kind == "files" {
		payload, err = crypt.EncryptFiles(files, key)
	} else {
		payload, err = crypt.EncryptPayload(plaintext, kind, name, key)
	}
	if err != nil {
		return err
	}
	meta := map[string]any{"v": 1}
	usePass := cfg.passphrase != ""
	if usePass {
		wrap, err := crypt.WrapKey(key, cfg.passphrase, pbkdf2Iterations)
		if err != nil {
			return err
		}
		meta["has_passphrase"] = true
		meta["pbkdf2"] = wrap
	}
	body, err := json.Marshal(map[string]any{
		"payload":   payload,
		"ttl":       cfg.ttl,
		"max_reads": cfg.maxReads,
		"kind":      kind,
		"meta":      meta,
	})
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Post(strings.TrimSuffix(cfg.server, "/")+"/api/drops",
		"application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("server unreachable: %w", err)
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusCreated:
	case http.StatusTooManyRequests:
		return errors.New("rate limited - slow down and retry")
	default:
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("server returned %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	var created struct {
		ID  string `json:"id"`
		URL string `json:"url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		return fmt.Errorf("bad server response: %w", err)
	}
	link := created.URL
	if usePass {
		fmt.Fprintln(os.Stderr, "passphrase required to open; share it over a separate channel")
	} else {
		link += "#" + crypt.EncodeKey(key)
	}
	fmt.Println(link)
	return nil
}

func runGet(args []string) error {
	var rawURL, out string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-o":
			i++
			if i >= len(args) {
				return errors.New("-o needs a value")
			}
			out = args[i]
		default:
			if rawURL != "" {
				return fmt.Errorf("unexpected argument %q", args[i])
			}
			rawURL = args[i]
		}
	}
	if rawURL == "" {
		return errors.New("usage: deadrop get <url> [-o file]")
	}

	id, fragment, base, err := parseDropURL(rawURL)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest(http.MethodPost, strings.TrimSuffix(base, "/")+"/api/drops/"+id+"/read", nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("server unreachable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return errors.New("not found - the secret was already read, expired, or never existed")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned %d", resp.StatusCode)
	}
	var body struct {
		Payload string          `json:"payload"`
		Kind    string          `json:"kind"`
		Meta    json.RawMessage `json:"meta"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return fmt.Errorf("bad server response: %w", err)
	}

	key, err := resolveKey(body.Meta, fragment)
	if err != nil {
		return err
	}
	env, err := crypt.DecryptPayload(body.Payload, key)
	if err != nil {
		return errors.New("decryption failed - wrong link, corrupted data, or wrong passphrase")
	}

	if env.V == crypt.VersionFiles || body.Kind == "files" {
		dir := out
		if dir == "" {
			dir = "."
		}
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return err
		}
		for _, f := range env.Files {
			path := filepath.Join(dir, filepath.Base(f.Name))
			if err := os.WriteFile(path, f.Data, 0o600); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "saved %s (%d bytes)\n", path, len(f.Data))
		}
		return nil
	}
	if body.Kind == "file" || env.Kind == "file" {
		path := out
		if path == "" {
			path = env.Name
			if path == "" {
				path = "secret.bin"
			}
		}
		if err := os.WriteFile(path, env.Data, 0o600); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "saved %s (%d bytes, link burned)\n", path, len(env.Data))
		return nil
	}
	fmt.Print(string(env.Data))
	if len(env.Data) > 0 && env.Data[len(env.Data)-1] != '\n' {
		fmt.Println()
	}
	return nil
}

// parseDropURL extracts id, fragment key and server base from the link.
func parseDropURL(rawURL string) (id, fragment, base string, err error) {
	u := rawURL
	if i := strings.Index(u, "#"); i >= 0 {
		fragment = u[i+1:]
		u = u[:i]
	}
	if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
		return "", "", "", errors.New("url must start with http:// or https://")
	}
	path := strings.TrimPrefix(strings.TrimPrefix(u, "http://"), "https://")
	slash := strings.Index(path, "/")
	if slash < 0 {
		return "", "", "", errors.New("url must contain /d/<id>")
	}
	host, rest := path[:slash], path[slash:]
	rest = strings.TrimSuffix(rest, "/")
	if !strings.HasPrefix(rest, "/d/") {
		return "", "", "", errors.New("url must contain /d/<id>")
	}
	id = strings.TrimPrefix(rest, "/d/")
	if id == "" || strings.Contains(id, "/") {
		return "", "", "", errors.New("invalid drop id in url")
	}
	scheme := "http://"
	if strings.HasPrefix(u, "https://") {
		scheme = "https://"
	}
	return id, fragment, scheme + host, nil
}

// resolveKey picks the data key: fragment first, else passphrase unwrap.
func resolveKey(meta json.RawMessage, fragment string) ([]byte, error) {
	if fragment != "" {
		return crypt.DecodeKey(fragment)
	}
	var m struct {
		HasPassphrase bool            `json:"has_passphrase"`
		PBKDF2        *crypt.MetaWrap `json:"pbkdf2"`
	}
	if len(meta) > 0 {
		if err := json.Unmarshal(meta, &m); err != nil {
			return nil, fmt.Errorf("bad meta: %w", err)
		}
	}
	if m.HasPassphrase && m.PBKDF2 != nil {
		pass := os.Getenv("CACHE22_PASSPHRASE")
		if pass == "" {
			fmt.Fprint(os.Stderr, "Passphrase: ")
			b, err := io.ReadAll(io.LimitReader(os.Stdin, 1024))
			if err != nil {
				return nil, err
			}
			pass = strings.TrimRight(string(b), "\r\n")
		}
		if pass == "" {
			return nil, errors.New("passphrase required (set CACHE22_PASSPHRASE)")
		}
		return crypt.UnwrapKey(*m.PBKDF2, pass)
	}
	return nil, errors.New("no key in link fragment and drop is not passphrase-protected")
}

func baseName(p string) string {
	if i := strings.LastIndexAny(p, `/\`); i >= 0 {
		return p[i+1:]
	}
	return p
}

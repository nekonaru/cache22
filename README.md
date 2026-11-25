<div align="center">
  <img src="assets/icon.svg" alt="Cache22 logo" width="120" />
  <h1>Cache22: One-Time Secret Sharing</h1>
  <p><b>Share one-time secrets that burn after reading,<br/>encrypted before they leave your device.</b></p>

  <a href="https://github.com/nekonaru/cache22/releases"><img src="https://img.shields.io/github/v/release/nekonaru/cache22?style=flat-square&color=blue" alt="Release" /></a>
  <a href="https://github.com/nekonaru/cache22/actions/workflows/ci.yml"><img src="https://github.com/nekonaru/cache22/actions/workflows/ci.yml/badge.svg?branch=main&style=flat-square" alt="CI" /></a>
  <img src="https://img.shields.io/badge/Go-1.23%2B-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go" />
  <img src="https://img.shields.io/badge/Platform-Linux%2C%20macOS%2C%20Windows-blue?style=flat-square" alt="Platform" />
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-MIT-green?style=flat-square" alt="License: MIT" /></a>
  <img src="https://img.shields.io/badge/Status-Active-brightgreen?style=flat-square" alt="Status" />
</div>

<br />

**English** | [Bahasa Indonesia](README.id.md)

> [!NOTE]
> Cache22 burns a secret after it is read. The server stores ciphertext only, and the key never leaves the sender's device.

---

## Table of Contents

- [About](#about)
- [Features](#features)
- [Requirements](#requirements)
- [Installation](#installation)
- [Usage](#usage)
- [API](#api)
- [Configuration](#configuration)
- [Project Structure](#project-structure)
- [Development](#development)
- [Testing](#testing)
- [Security](#security)
- [Continuous Integration](#continuous-integration)
- [FAQ](#faq)
- [Author](#author)
- [License](#license)

---

## About

**Cache22** shares text or a small file as a one-time secret. You paste it, get a unique link, and send it. The person who opens it sees the content once; it is then permanently deleted from the server. Encryption happens on the sender's device, so the service never sees what you shared.

> _"One link, one read, then nothing."_

---

## Features

### Sending

| Feature | Description |
|---------|-------------|
| **One-time links** | Paste text or a small file, get a unique link that burns after reading |
| **Text secrets up to 64 KB** | Paste directly in the UI |
| **File secrets up to 2 MB** | Filename is encrypted along with the file |
| **Configurable expiry** | 10 minutes, 1 hour, 24 hours or 7 days |
| **Limited opens** | Read once by default; optional limit of 1 to 10 opens |

### Security

| Feature | Description |
|---------|-------------|
| **Zero-knowledge crypto** | AES-256-GCM in the browser; the key stays in the URL fragment |
| **Optional passphrase lock** | PBKDF2-HMAC-SHA256, 600,000 iterations; the link alone is useless without it |
| **Uniform failures** | Unknown, read, expired and malformed ids all return the same `404` |
| **Atomic burn** | Read and delete happen under one lock; concurrent readers cannot both get the secret |

### Privacy

| Feature | Description |
|---------|-------------|
| **No server plaintext** | The server stores ciphertext and a small opaque meta object only |
| **Key never sent** | The fragment is stripped from the address bar on page load |
| **Safe rendering** | Secrets render via `textContent`; CSP uses a per-request nonce and forbids inline scripts |
| **Minimal logging** | Logs carry method, route template, status and duration only |

### The CLI

| Feature | Description |
|---------|-------------|
| **`deadrop` CLI** | Pipe a secret from the terminal and get a link back |
| **Read and burn** | `deadrop get <url>` decrypts, prints or saves the file, then burns the drop |
| **Web-compatible format** | Links created by the CLI open in the web UI, and the other way around |

### Hosting

| Feature | Description |
|---------|-------------|
| **Single static binary** | Go standard library only; the UI is embedded in the binary |
| **Docker image** | Distroless, non-root image ready behind your reverse proxy |
| **Persistent storage by default** | bbolt database; survives restarts; `-data -` switches to ephemeral in-memory mode |
| **Gzip compression** | Pages and API responses serve as gzip when the client accepts it (about a third of the bytes) |
| **Read receipt webhook** | Optional -webhook / CACHE22_WEBHOOK_URL POST on every read; SSRF-guarded |
| **Telegram read receipt** | Optional -telegram-bot-token + -telegram-chat-id; push message via the Telegram Bot API |
| **Per-IP rate limiting** | Built-in token bucket, tuned by flag or env |

### The interface

| Feature | Description |
|---------|-------------|
| **Bilingual UI** | English and Indonesian, detected from the browser |
| **Dark and light** | Follows the system color scheme |
| **Responsive** | Works on phones and desktops |

---

## Requirements

| Dependency | Minimum Version | Notes |
|------------|-----------------|-------|
| **Go** | 1.23 | Only to build from source |
| **Git** | Any | Only to build from source |
| **Docker** | 24 | Only to run the container image |
| **A modern browser** | Web Crypto support | Chrome, Firefox, Safari, Edge (recent versions) |

> [!TIP]
> If you only want to use Cache22, you do not need Go or Docker. Download the archives from the [Releases](https://github.com/nekonaru/cache22/releases) page or `go install` it (see Installation) instead.

### Never used a terminal? Start here

<details>
<summary><b>How to open a terminal</b></summary>

1. Windows: press the **Windows** key, type `PowerShell`, then press **Enter**. A blue window opens; type commands there.
2. macOS: press **Command + Space**, type `Terminal`, then press **Return**.
3. Linux: press **Ctrl + Alt + T**, or search for `Terminal` in your application menu.

</details>

---

## Installation

### Option A: `go install` (easiest)

Requires Go 1.23 or newer.

```bash
go install github.com/nekonaru/cache22/cmd/cache22@latest
go install github.com/nekonaru/cache22/cmd/deadrop@latest
cache22
```

Open <http://localhost:8080>. Prebuilt archives for Linux, macOS and Windows are also attached to each [release](https://github.com/nekonaru/cache22/releases); each archive contains `cache22`, `deadrop`, the README and the license.

### Option B: Build from source (for development)

#### 1. Get the project code

<details>
<summary><b>I have Git installed</b></summary>

```bash
git clone https://github.com/nekonaru/cache22.git
cd cache22
```

</details>

<details>
<summary><b>I don't have Git</b></summary>

1. Open the repository page on GitHub.
2. Click the green **Code** button, choose **Download ZIP**.
3. Extract the ZIP and open a terminal in the extracted folder.

</details>

#### 2. Install dependencies and verify

```bash
go vet ./...
go test -race ./...
```

> [!IMPORTANT]
> `go vet` must report no issues and `go test -race ./...` must pass before you rely on a build.

#### 3. Build and run

```bash
make build     # bin/cache22 and bin/deadrop
./bin/cache22  # listens on :8080
```

### Option C: Render.com (one click, free)

The repository ships a `render.yaml` Blueprint. Open https://render.com/deploy?repo=https://github.com/nekonaru/cache22, sign in, and approve the blueprint. Render builds the Dockerfile into a free web service; `/healthz` is wired as the health check.

The free plan has an ephemeral filesystem, so the service runs with `-data -` (pure in-memory storage). After first deploy, set `CACHE22_BASE_URL` to your service URL (for example `https://cache22-xxxx.onrender.com`) under **Environment**, because created links embed it.

> [!NOTE]
> Render free services sleep after 15 minutes of idle traffic and take about a minute to wake on the next request. The first visit after idle may be slow.

---

## Usage

### First launch

1. Run `cache22` (or `make run` for development).
2. Open <http://localhost:8080>.
3. Paste your secret, pick an expiry, and click **Create link**.
4. Send the link, plus the passphrase separately if you enabled one.
5. Once the recipient opens the link, the secret is burned.

> [!NOTE]
> The link contains the decryption key in its `#fragment`. Whoever has the full link can read the secret until it is burned.

### Everyday use

- **Paste a secret**: typing `token: abc123` in the box is enough.
- **Attach a file**: click the drop zone to pick a file up to 2 MB; the filename is encrypted too.
- **Lock with a passphrase**: enable the toggle, then share the passphrase over another channel. The link alone is useless without it.
- **Limit opens**: change "Max opens" from 1 to 10.
- **From the terminal**: `echo "db-password: hunter2" | deadrop` prints the link.
- **Read back**: `deadrop get <url>` prints text secrets or saves files with `0600` permissions.

---

## API

| Method and path | Purpose |
|-----------------|---------|
| `POST /api/drops` | Create a drop. Body: `{payload, ttl, max_reads?, kind, meta?}`. Returns `201 {id, url, expires_at}` |
| `GET /d/{id}` | Recipient page. Always `200`, identical for every id |
| `POST /api/drops/{id}/read` | Consume a drop. Returns `200 {payload, kind, meta}` or the uniform `404 {"error":"not_found"}` |
| `GET /healthz` | Health check. Returns `200 ok` |

`payload` is base64url of `iv[12]` plus an AES-256-GCM ciphertext of a JSON envelope `{"v":1,"kind":"text|file","name"?,"data":base64url}`. `ttl` is one of `10m`, `1h`, `24h`, `7d`. `max_reads` defaults to `1`, maximum `10`. Ids are 128-bit random, base64url (22 characters). Errors: `400`, `413`, `429`, `503`.

---

## Configuration

| Flag | Env | Default |
|------|-----|---------|
| `-addr` | `CACHE22_ADDR` | `:8080` |
| `-base-url` | `CACHE22_BASE_URL` | `http://localhost:8080` |
| `-data` | `CACHE22_DATA` | `cache22.db` |
| `-max-text-bytes` | `CACHE22_MAX_TEXT_BYTES` | `65536` |
| `-max-file-bytes` | `CACHE22_MAX_FILE_BYTES` | `2097152` |
| `-max-total-bytes` | `CACHE22_MAX_TOTAL_BYTES` | `67108864` |
| `-rate-per-min` | `CACHE22_RATE_PER_MIN` | `30` |
| `-burst` | `CACHE22_BURST` | `10` |
| `-trust-proxy` | `CACHE22_TRUST_PROXY` | `false` |
| `-webhook` | `CACHE22_WEBHOOK_URL` | _(empty)_ |
| `-telegram-bot-token` | `CACHE22_TELEGRAM_BOT_TOKEN` | _(empty)_ |
| `-telegram-chat-id` | `CACHE22_TELEGRAM_CHAT_ID` | _(empty)_ |
| `-notify-secret` | `CACHE22_NOTIFY_SECRET` | _(empty)_ |

`-trust-proxy` makes the server honor `X-Forwarded-For` for rate limiting; enable it only behind a trusted reverse proxy. Every response carries `Cache-Control: no-store`, `Referrer-Policy: no-referrer`, `X-Content-Type-Options: nosniff`, `X-Robots-Tag: noindex` and a strict `Content-Security-Policy` with a per-request nonce.

`-webhook` fires a read receipt POST to the given URL whenever a drop is read. The body is JSON: `{"event":"drop.read","id":"...","kind":"text","reads_left":0,"read_at":"..."}`. URLs are validated before every send: the host must be public and none of its resolved addresses may be private, loopback, link-local or multicast. Anything else is dropped with an error log. The receipt never contains the ciphertext, key or payload.

`-telegram-bot-token` and `-telegram-chat-id` fire a Telegram message to a bot chat on the same events when both are set. The message carries only metadata (drop event, kind, reads left, read time), never the payload. The bot token rides in the URL path only; redirects are refused.

`-notify-secret` enables an HMAC-SHA256 signature on webhook receipts. The header `X-Cache22-Signature: sha256=<hex>` covers the exact JSON body bytes. A receiver with the same secret can reject forged or modified receipts. The secret is never logged or returned.

`-data` points to the bbolt database file; set it to `-` to force pure in-memory storage (no persistence). The database is created with owner-only permissions (`0600`); its rows contain only ciphertext and passphrase-wrapped meta, never plaintext or keys.

---

## Project Structure

```
cache22/
├── cmd/
│   ├── cache22/              # server entrypoint
│   └── deadrop/              # the CLI
├── internal/
│   ├── config/               # flags and env, validated
│   ├── crypt/                # CLI-side crypto, mirrors the browser
│   ├── ratelimit/            # per-IP token bucket
│   ├── server/               # handlers, headers, logging
│   ├── telegram/             # Telegram bot read receipts
│   ├── webhook/              # read receipt client, SSRF-guarded
│   └── store/                # Store: interface, in-memory and bbolt
├── web/
│   ├── index.html            # the single-page UI
│   └── web.go                # go:embed
├── assets/                   # logos, mark, icon, favicon
├── README.md                 # English documentation (source of truth)
├── README.id.md              # Indonesian documentation
├── CHANGELOG.md              # English changelog
├── CHANGELOG.id.md           # Indonesian changelog
├── Dockerfile                # distroless, non-root
└── Makefile                  # build, test, run, docker
```

---

## Development

### Changing the UI text

1. Edit the `en` and `id` objects at the bottom of `web/index.html` together.
2. Run `go build ./...` and reload the page with DevTools closed.

### Adding a flag or env var

1. Add it to `internal/config/config.go` with a default.
2. Use it from `internal/server` or `cmd/cache22`.
3. Document it in the Configuration table of both READMEs.

### Releases

1. Bump the version in `CHANGELOG.md` and `CHANGELOG.id.md`.
2. Tag `vX.Y.Z` on `main` after CI is green.
3. CI builds the archives; the release workflow attaches them with a `SHA256SUMS.txt`.

> [!WARNING]
> CI enforces `gofmt`. Do not hand-align code; run `gofmt -w .` before committing.

---

## Testing

Verification before every push, both must be clean:

```bash
go vet ./...
go test -race ./...
```

| Check | Requirement |
|-------|-------------|
| `go vet ./...` | Zero issues |
| `go test -race ./...` | All tests pass, including the 100-goroutine race test and the JS/Go wire-format vector |

---

## Security

Cache22 was built around these verified properties: encryption with AES-256-GCM on the sender's device, no plaintext or key material on the server or in logs, uniform `404` responses for every read failure, atomic read-and-burn under one mutex, a strict nonce-based Content Security Policy, and a per-IP token-bucket rate limiter. `gofmt`, `go vet` and `go test -race` run on every push.

> [!NOTE]
> Report vulnerabilities privately as described in [`SECURITY.md`](SECURITY.md). Do not open a public issue for them.

---

## Continuous Integration

| Workflow | Purpose |
|----------|---------|
| `ci.yml` | `gofmt`, `go vet`, `go test -race`, build server and CLI, Docker image build |

CI runs on push to `main`, on pull requests and on `v*` tags.

---

## FAQ

<details>
<summary><b>Can the server read my secret?</b></summary>

No. The secret is encrypted in your browser with AES-256-GCM before it is sent. The decryption key stays in the URL fragment (`#...`) and is never transmitted. The server stores ciphertext and a small opaque metadata object.
</details>

<details>
<summary><b>Can the recipient keep a copy?</b></summary>

Cache22 deletes the secret after it is read, but it cannot control what the recipient copies, pastes or photographs.
</details>

<details>
<summary><b>Does the secret survive a server restart?</b></summary>

It survives a restart: drop data lives on disk in the bbolt database (owner-only permissions, ciphertext only). If you run with `-data -`, everything is held in memory and a restart wipes it.
</details>

<details>
<summary><b>Why do wrong links and burnt drops look the same?</b></summary>

They return byte-identical `404 {"error":"not_found"}` responses, so nobody can tell whether an id exists, was read, or expired.
</details>

<details>
<summary><b>Why not Argon2 for passphrases?</b></summary>

The Web Crypto API does not provide Argon2, so Cache22 uses PBKDF2-HMAC-SHA256 with 600,000 iterations and a random salt per secret.
</details>

---

## Author

<div align="center">

| [![Nicolas Dwi Dharma](https://github.com/nekonaru.png?size=100)](https://github.com/nekonaru) |
|:---:|
| **Nicolas Dwi Dharma** |
| [github.com/nekonaru](https://github.com/nekonaru) |

</div>

---

## License

Distributed under the **MIT License**. See [`LICENSE`](LICENSE) for more information.

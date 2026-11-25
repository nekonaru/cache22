# Changelog

All notable changes to Cache22 are documented here. The English file is the source of truth; `CHANGELOG.id.md` is the Indonesian translation.

## v1.5.0 - 2026-11-26

Feature release: signed webhook receipts plus a stricter verification pipeline.

### New features

- **Signed webhook receipts.** New `-notify-secret` / `CACHE22_NOTIFY_SECRET` flag (off by default) adds `X-Cache22-Signature: sha256=<HMAC of the raw JSON body>` so a receiver can verify each read receipt.

### Improvements

- **Verification pipeline.** Added a pre-commit hook, a local `scripts/verify.sh` and a 60 percent coverage gate in CI; total coverage is now 66 percent.
- **Tests.** Added config parsing tests (100 percent coverage) and CLI flag tests (35 percent coverage).

### Known limitations

- **Telegram receipts stay unsigned.** The Bot API has no receiver verification surface; use the webhook with a secret when authenticity matters.
- **Receipts remain best-effort.** Neither channel blocks the read response or retries after failure.

## v1.4.0 - 2026-11-19

## v1.4.0 - 2026-11-19

Feature release: Telegram read receipts. Operators who already use the webhook now have a second option that lands in their Telegram.

### New features

- **Telegram read receipts.** New `-telegram-bot-token` / `CACHE22_TELEGRAM_BOT_TOKEN` and `-telegram-chat-id` / `CACHE22_TELEGRAM_CHAT_ID` flags (off by default). After a successful read, the server POSTs a short message to the Telegram Bot API in the configured chat. Message carries only metadata: event, kind, reads left, read time.
- **Both notifiers can run.** Webhook and Telegram both fire on the same read when both are configured; both are best-effort with a 2.2s client timeout and never block the response.

### Known limitations

- **Receipt is unsigned and best-effort.** Both channels report that a read happened; neither can prove identity.
- **Bot token is a credential.** `CACHE22_TELEGRAM_BOT_TOKEN` stays in environment only; never committed, never logged, never echoed by the server.

## v1.3.1 - 2026-11-18

## v1.3.1 - 2026-11-18

Patch to the Render Blueprint: Render injects PORT=10000 by default, so the free service would have failed to come up behind Render's load balancer.

### Bug fixes

- **Render Blueprint.** `CACHE22_ADDR` is now `:10000` to match the port Render exposes for web services by default; the service listens there instead of `:8080` when deployed via the Blueprint.

## v1.3.0 - 2026-11-18

## v1.3.0 - 2026-11-18

Deployment release: the repo now ships a Render.com Blueprint so anyone can deploy Cache22 with one click.

### Improvements

- **Render Blueprint.** `render.yaml` builds the existing Dockerfile into a free web service with `/healthz` wired as the health check. README and README.id.md document per-request ephemera note (free filesystem) and how to set `CACHE22_BASE_URL` after first deploy.
- **Docs.** Installation section now documents Option A, B and C with identical content in both languages.

## v1.2.0 - 2026-11-17

Feature release: optional read receipts for operators who want to know when a drop is read.

### New features

- **Webhook read receipts.** New `-webhook` / `CACHE22_WEBHOOK_URL` flag (off by default): after a successful read, the server POSTs a JSON event (`drop.read`, id, kind, reads_left, read_at) to the configured URL. Delivery is best effort with a 3 second timeout; failures are logged without the id.
- **SSRF guard on the webhook.** Endpoint validation rejects non-http(s) schemes and any host that resolves to a private, loopback, link-local, CGNAT, multicast or unspecified address, plus all redirects. Payload contains no ciphertext, key or plaintext.

### Known limitations

- **Receipt is one way and unauthenticated.** The webhook URL is operator-controlled but the payload is not signed; treat the endpoint as untrusted for proof, not proof of identity.
- **Copy protection.** The service cannot stop the recipient from copying or photographing the plaintext after reveal.

## v1.1.0 - 2026-11-15

Persistent storage by default: drops now survive a restart via a bbolt database file (default `cache22.db`, owner-only permissions), completing the roadmap item for persistent storage.

### Improvements

- **Storage.** New `-data` / `CACHE22_DATA` flag selects a persistent bbolt file (default `cache22.db`); `-` forces pure in-memory mode as before. `Consume` now happens inside a single bbolt write transaction. Docker image sets `CACHE22_DATA=/data/cache22.db` and ships a `VOLUME ["/data"]` owned by the non-root user.
- **Dockerfile.** Now copies `go.sum` and creates `/data` owned by the non-root user; `gitignore` covers `cache22.db`.

### Known limitations

- **Restart no longer loses drops** (unless `-data -` is set explicitly).
- **A live link with its `#fragment` is a read capability** until the drop is burned; the passphrase lock exists for this.
- **Cache22 cannot stop the recipient** from copying or photographing the plaintext after reveal.

## v1.0.3 - 2026-11-15

Performance and hardening release: responses now gzip to about a third of the original size and the passphrase reveal flow no longer loses the secret before the passphrase is entered.

### Bug fixes

- **Web UI.** For passphrase protected drops, the first click previously consumed the drop but the user could not retry the passphrase after a typo; the payload is now held across clicks so the passphrase can be retried without a refetch. The misleading error for rate limited reads is now a toast, not a fake "already read" message.

### Improvements

- **Gzip.** HTML and JSON responses are served gzip compressed when the client accepts it (10.3 KB wire for the 31 KB page, Vary: Accept-Encoding).
- **Security headers.** Added Cross-Origin-Opener-Policy, Cross-Origin-Resource-Policy and Permissions-Policy on every response. HSTS is emitted only behind -trust-proxy.

## v1.0.2 - 2026-11-15

Fix and documentation release: the Content Security Policy no longer blocks the page, and the repository documentation follows the project documentation standard.

### Bug fixes

- **Web UI.** Inline style attributes violated the nonce based CSP and were rejected by the browser, so parts of the page rendered unstyled and the console filled with errors. All dynamic styles now come from the stylesheet. Verified in DevTools: zero console errors on load, create, reveal and burn flows.
- **Page footnote.** The inline `style` attribute on the no-JS fallback is now a stylesheet class.
- **Makefile.** `make test` runs `go vet` first so local runs match CI.

### Improvements

- **README.** Both files restructured to the standard section order with synchronized language content and a table of contents.
- **Changelog.** Split into English and Indonesian files with matching content.
- **Repository metadata.** Description is one sentence without tooling names; topics reviewed against the project scope.

## v1.0.1 - 2026-11-15

Robustness patch on top of v1.0.0.

### Bug fixes

- **CLI.** Dangling create flags (for example `-ttl` with no value) exit with a clean error instead of panicking with index out of range.
- **Server.** The CSP nonce response writer now implements `http.Flusher`, so streaming paths cannot be broken by the middleware chain.
- **Web.** The page declares a `<noscript>` fallback explaining that JavaScript is required for all crypto.

## v1.0.0 - 2026-11-15

First public release of Cache22: one-time secrets that burn after reading, with zero-knowledge client-side encryption.

### New features

- **One-time drops.** Create (`POST /api/drops`), read once and burn (`POST /api/drops/{id}/read`), with TTL `10m`, `1h`, `24h` and `7d`.
- **Uniform failures.** Unknown, already read, expired and malformed ids all return the same `404 {"error":"not_found"}`.
- **Atomic burn.** Read and delete happen under one mutex; a 100-goroutine test proves exactly one reader wins.
- **Zero-knowledge encryption.** AES-256-GCM in the browser, key only in the URL fragment, never sent to the server.
- **Passphrase lock.** Optional PBKDF2-HMAC-SHA256 wrapping of the data key (600,000 iterations); the link alone is useless without the passphrase.
- **Encrypted file drops.** Files up to 2 MB, filename included in the encrypted envelope.
- **Embedded web UI.** Bilingual (English, Indonesian), dark and light, strict CSP with per-request nonce, secrets rendered via `textContent`.
- **`deadrop` CLI.** Pipe secrets from the terminal, `deadrop get` burns after reading, wire format byte-compatible with the web UI.
- **Rate limiting and expiry sweeper.** Per-IP token bucket, one minute sweep interval, `/healthz`, graceful shutdown.
- **Distroless non-root Docker image, MIT license, bilingual README, CI.**

### Known limitations

- **In-memory storage.** A restart deletes all drops; persistent storage is tracked on the roadmap.
- **Copy protection.** The service cannot stop the recipient from copying or photographing the secret after reading.

[v1.5.0]: https://github.com/nekonaru/cache22/compare/v1.4.0...v1.5.0
[v1.4.0]: https://github.com/nekonaru/cache22/compare/v1.3.1...v1.4.0
[v1.3.1]: https://github.com/nekonaru/cache22/compare/v1.3.0...v1.3.1
[v1.3.0]: https://github.com/nekonaru/cache22/compare/v1.2.0...v1.3.0
[v1.2.0]: https://github.com/nekonaru/cache22/compare/v1.1.0...v1.2.0
[v1.1.0]: https://github.com/nekonaru/cache22/compare/v1.0.3...v1.1.0
[v1.0.3]: https://github.com/nekonaru/cache22/compare/v1.0.2...v1.0.3
[v1.0.2]: https://github.com/nekonaru/cache22/compare/v1.0.1...v1.0.2
[v1.0.1]: https://github.com/nekonaru/cache22/compare/v1.0.0...v1.0.1
[v1.0.0]: https://github.com/nekonaru/cache22/releases/tag/v1.0.0

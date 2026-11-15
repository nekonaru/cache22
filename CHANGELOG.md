# Changelog

All notable changes to Cache22 are documented here. The English file is the source of truth; `CHANGELOG.id.md` is the Indonesian translation.

## v1.0.2 - 2026-11-15

Fix and documentation release: the Content Security Policy no longer blocks app rendering, and the repository documentation follows the project documentation standard.

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

[v1.0.2]: https://github.com/nekonaru/cache22/compare/v1.0.1...v1.0.2
[v1.0.1]: https://github.com/nekonaru/cache22/compare/v1.0.0...v1.0.1
[v1.0.0]: https://github.com/nekonaru/cache22/releases/tag/v1.0.0

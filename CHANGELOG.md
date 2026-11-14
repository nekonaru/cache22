# Changelog

All notable changes to Cache22 are documented here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project uses [Semantic Versioning](https://semver.org/).

## [1.0.0] — 2026-11-15

First public release.

### Added

- One-time secret drops: create (`POST /api/drops`), read-and-burn (`POST /api/drops/{id}/read`), with TTL `10m`/`1h`/`24h`/`7d`.
- Uniform failure surface: unknown, already-read, expired and malformed ids all return byte-identical `404 {"error":"not_found"}`.
- Atomic read-and-delete under a single mutex; optional `max_reads` 1–10.
- Zero-knowledge client-side encryption: AES-256-GCM, 256-bit key from `crypto.getRandomValues`, 12-byte IV, key carried in the URL fragment only.
- Optional passphrase lock: PBKDF2-HMAC-SHA256 (600 000 iterations) wrapping the data key; wrapping parameters stored as opaque server-side meta.
- Encrypted file drops up to 2 MB (filename included in the encrypted envelope); text drops up to 64 KB.
- Embedded single-page UI (`web/index.html`) — bilingual ID/EN, dark and light, fully self-contained, strict CSP with per-request nonce.
- `deadrop` CLI: pipe secrets from the terminal, `deadrop get` to read-and-burn, byte-compatible wire format with the web UI (verified by a cross-implementation test vector).
- Per-IP token-bucket rate limiting (self-contained), strict security headers on every response, minimal structured logs (method, route template, status, duration — never ids, payloads, or fragments).
- `GET /healthz`, graceful shutdown, expiry sweeper (1-minute interval), in-memory storage with a global memory budget (`503` when full).
- MIT license, bilingual README, distroless non-root Dockerfile, Makefile.

### Security

- IDs are 128-bit `crypto/rand` (base64url, 22 chars).
- `GET /d/{id}` never consumes a drop and never touches storage; link-preview bots cannot burn secrets.
- Recipient page renders secrets via `textContent` only.

### Known limitations

- In-memory storage: restart loses all drops.
- Cannot prevent the recipient from copying the secret after reading.

[1.0.0]: https://github.com/nekonaru/cache22/releases/tag/v1.0.0

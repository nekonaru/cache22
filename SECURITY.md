# Security Policy

## Reporting a vulnerability

Please **do not** open a public GitHub issue for security vulnerabilities in Cache22.

Instead, email **nicolasdwidharma@gmail.com** with:

- a description of the vulnerability and its impact,
- steps to reproduce (proof of concept if possible),
- the commit or version affected.

You should receive an acknowledgement within 72 hours. Once the issue is confirmed and fixed, a security note will be published in the [release notes](https://github.com/nekonaru/cache22/releases) and `CHANGELOG.md`.

## Scope

Cache22 is a zero-knowledge, burn-after-reading secret sharing service. Reports of particular interest:

- flaws in the client-side encryption flow (key handling, IV reuse, PBKDF2 parameters),
- ways to read a drop without consuming it, or to consume it twice,
- server-side leaks of plaintext, ids in logs, or fragment material,
- SSRF or abuse primitives in the API.

## Supported versions

| Version | Supported |
| --- | --- |
| v1.0.x | âœ… |
| < v1.0.0 | âŒ |

## Safe use

Cache22 cannot prevent the secret's recipient from copying or photographing what they read. Links are capabilities: anyone holding the full link (including the `#fragment`) can read the secret until it is burned. Share links over a single trusted channel.


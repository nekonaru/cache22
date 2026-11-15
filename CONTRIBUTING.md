# Contributing to Cache22

Thanks for your interest! Cache22 is a small, security-sensitive codebase - contributions are welcome, but the bar for correctness is high.

## Getting started

```bash
git clone https://github.com/nekonaru/cache22.git
cd cache22
make test    # go vet + go test -race ./...
make run     # dev server on :8080
```

Requires Go ≥ 1.23. No third-party dependencies - please keep it that way unless the change is discussed in an issue first.

## Ground rules

1. `go vet ./...` and `go test -race ./...` must pass.
2. Never log ids, payloads, or URL fragments - only method, route template, status, duration.
3. `GET /d/{id}` must never consume a drop or return different bytes for different ids.
4. All read failures stay byte-identical: status `404`, body `{"error":"not_found"}`.
5. Consumption stays atomic.
6. Client-side crypto uses Web Crypto only; no third-party crypto libraries in the browser.
7. New behavior ships with tests.

## Pull requests

- Small, focused PRs beat large ones.
- Describe *why*, not just *what*.
- Commit format is `type: short description` (types: `feat`, `fix`, `docs`, `chore`, `refactor`, `test`, `perf`, `style`, `revert`), English, under 72 characters.
- Update `CHANGELOG.md` and `CHANGELOG.id.md` together under the top version or an `Unreleased` section.
- Keep user-facing changes bilingual: change `README.md` first, then `README.id.md`, in the same commit. Same sections, same order, same table rows.

## License

By contributing you agree your work is licensed under the [MIT License](LICENSE).

# Project

| Variable | Value |
|----------|-------|
| PROJECT_NAME | Cache22 |
| REPO | nekonaru/cache22 |
| AUTHOR_NAME | Nicolas Dwi Dharma |
| AUTHOR_GITHUB | nekonaru |
| PROJECT_KIND | web |
| STACK | Go (standard library), embedded single-page frontend (HTML/CSS/JS, Web Crypto) |
| PLATFORMS | Linux, macOS, Windows (server binary); any modern browser (web UI) |
| MIN_OS | n/a |
| DEFAULT_BRANCH | main |
| LICENSE | MIT |
| UI_LANGUAGES | English, Bahasa Indonesia |
| VERIFICATION | local-and-ci |
| PIPELINE | pr |

Assumptions: `PROJECT_KIND` is `web` (Go HTTP service with a browser UI) even though the repo also ships the `deadrop` CLI; the CLI is part of the same product, not a separate app. `STACK` is recorded as Go plus web per the `go.mod` and the embedded frontend.

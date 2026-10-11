# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

- `make test`: run all tests (`go test ./...`)
- `go test ./internal/web/ -run TestReadGated`: run one test
- `make race`: tests with the race detector (the `internal/web` suite takes about a minute this way)
- `make lint`: gofmt check, `go vet`, staticcheck, govulncheck
- `make run`: build and serve on http://localhost:8080 with a local database in `./data` (account `admin` / `change-me-please`)
- `make docker`: build the image

Static files and templates are embedded at build time, so restart the server after changing them.

## Documents

- Design spec: `docs/superpowers/specs/2026-10-11-pastr-design.md` (behaviour, routes, data model, security rules)
- Product record: `PRODUCT.md`; visual system: `DESIGN.md`

## Architecture

Request path: `cmd/pastr` wires `config` → `store` (SQLite) → `web.Server`. Every request passes `recoverer → securityHeaders → limitBody → loadSession → protect` (in `internal/web/middleware.go`) before the router in `server.go`. `protect` enforces the Origin check and the per-session CSRF token on every non-GET request.

- `internal/store` is the only package with SQL. `ConsumeView` counts a view, re-checks expiry, limit and owner, and deletes a paste that just used its last view, all in one transaction. `ReadPaste` is the uncounted public lookup.
- `internal/auth`: Argon2id hashing (concurrency-capped), sessions stored as SHA-256 of the token, and an in-memory `Throttle` (`Attempt` reserves a slot, `Release` hands it back).
- `internal/paste`: slug, signal colour, expiry and view-limit rules, and `IsPreviewCrawler`.
- `internal/sanitize`: the HTML allowlist, applied once when a paste is saved.
- `internal/web`: handlers, templates and embedded static files. Templates are parsed per page together with `base.html`.

## What pastr is

A lean, self-hosted pastebin for prose and spoilers: authors share text behind a URL so a film's plot twist never appears in the social post linking to it. Open source (MIT).

## Decided stack

Go, server-rendered (`net/http`, `html/template`), SQLite via `modernc.org/sqlite` (no CGO), `bluemonday` sanitiser, Argon2id passwords, Trix editor vendored. One static binary in a distroless Docker image. No Node toolchain.

## Rules that are easy to break

- The public read page (`/p/{slug}`) ships no JavaScript except the optional Umami tracker, and its body text is never animated or delayed.
- The paste body must never reach link-preview crawlers or preview metadata.
- Paste bodies are sanitised on write; only `internal/store` contains SQL.
- Expired, burned, deleted, unknown and not-yours pastes all return the same gone page with 404.
- No reader data or IP addresses are stored.
- Templates must not use inline `style` or `<script>`; the Content Security Policy blocks both. Only the editor pages get a style nonce, for Trix.
- An empty `{{define}}` does not override a `{{block}}` in `html/template`. Pages that drop the navigation or the events script set `NoNav` / `NoScript` on `pageData`.
- Paste URLs are secrets: log the route pattern, never the path.
- Lean by intent: features outside the spec need a reason.

## Working agreements

- UI and UX decisions go through the Impeccable skill; the chosen direction is "Poster Wall" (see spec section 8).
- Code reviews and security audits use Octopus at the checkpoints in spec section 15.
- Temporary files stay out of the repository; `.impeccable/` is ignored.
- Ask before creating the GitHub repository or pushing.

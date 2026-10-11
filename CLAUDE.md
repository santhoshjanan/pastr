# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Status

Design is written; no code exists yet, so there are no build, lint or test commands. Add them here as soon as they exist.

- Design spec: `docs/superpowers/specs/2026-10-11-pastr-design.md` (source of truth for behaviour, routes, data model and security rules)
- Product record: `PRODUCT.md`

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
- Lean by intent: features outside the spec need a reason.

## Working agreements

- UI and UX decisions go through the Impeccable skill; the chosen direction is "Poster Wall" (see spec section 8).
- Code reviews and security audits use Octopus at the checkpoints in spec section 15.
- Temporary files stay out of the repository; `.impeccable/` is ignored.
- Ask before creating the GitHub repository or pushing.

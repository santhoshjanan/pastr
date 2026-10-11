# pastr design

Date: 2026-10-11
Status: approved and built. Section 16 lists where the build differs from the text below.

## 1. Purpose

pastr shares text behind a URL so that a spoiler never appears in the social post that links to it. The owner discusses films on X.com and wants to post a link instead of the plot twist.

Success means:

- A reader goes from tapping the link to reading with no friction, on a phone, on mobile data.
- An author goes from idea to copied link in under a minute.
- The link preview card never leaks the paste body.

Guiding constraints, in priority order when they conflict:

1. Nothing slows the read.
2. Security and basic privacy.
3. Lean: every feature must earn its place.
4. A clean, easily self-hosted, open-source codebase.

## 2. Scope

### In scope

- Multi-user accounts with username and password sign-in.
- Sign-ups switchable by the admin, off by default.
- Create, list, edit and delete pastes, each owned by one user.
- Public, unlisted read page per paste.
- Per paste: title, rich-text body, reveal mode, optional expiry, optional view limit.
- Optional author byline (display name and link).
- Optional Umami analytics with custom events.
- Single Docker image with SQLite storage.

### Out of scope

- JSON or command-line API.
- Images, file uploads, embeds, code-paste features, syntax highlighting.
- Public index, search, tags, comments, reactions.
- Email of any kind, including self-service password reset.
- OAuth, passkeys, two-factor authentication.
- Theme toggle (themes follow the system setting).
- "Read to the end" scroll tracking.
- Browser end-to-end test suite.
- Bundled Umami or bundled reverse proxy.

## 3. Architecture

One Go binary serves public read pages, the authenticated app and static assets embedded at build time. One SQLite file in WAL mode sits on a Docker volume. There are no other processes.

Stack: Go standard library `net/http` and `html/template`; `modernc.org/sqlite` (pure Go, so the binary is static with CGO disabled); `bluemonday` for HTML sanitising; `golang.org/x/crypto/argon2` for password hashing. The rich-text editor is Trix, vendored.

### Packages

| Package | Responsibility | Depends on |
|---|---|---|
| `cmd/pastr` | Entry point: load config, open database, start server; `healthcheck` subcommand | all below |
| `internal/config` | Read and validate environment variables | none |
| `internal/store` | SQLite access and migrations; the only package containing SQL | none |
| `internal/auth` | Password hashing, sessions, login throttling, CSRF tokens | `store` |
| `internal/paste` | Paste rules: slug generation, expiry, view limits, crawler detection, signal colour | `store` |
| `internal/sanitize` | HTML allowlist and normalisation applied to every body on save | none |
| `internal/web` | Routes, middleware, handlers, templates, embedded assets | `auth`, `paste`, `sanitize`, `config` |

Migrations are embedded SQL files applied in order at start-up, tracked in a `schema_migrations` table.

## 4. Data model

### `users`

| Column | Notes |
|---|---|
| `id` | Integer primary key |
| `username` | Unique, 3 to 32 characters, `a-z 0-9 _ -`, stored lower-case |
| `password_hash` | Argon2id, encoded with its parameters |
| `display_name` | Optional, up to 60 characters |
| `display_link` | Optional, `http` or `https` URL |
| `is_admin` | Boolean |
| `disabled` | Boolean |
| `created_at` | UTC timestamp |

### `sessions`

| Column | Notes |
|---|---|
| `token_hash` | SHA-256 of the session token; primary key |
| `user_id` | Foreign key, cascade delete |
| `csrf_token` | Random, per session |
| `expires_at` | 30 days after creation |
| `created_at` | UTC timestamp |

Only the hash of the token is stored, so a leaked database file cannot be used to hijack sessions.

### `pastes`

| Column | Notes |
|---|---|
| `slug` | Primary key; 10 random base62 characters (about 59 bits) |
| `user_id` | Foreign key, cascade delete |
| `title` | Required, up to 120 characters, plain text |
| `body_html` | Sanitised HTML, up to 100 KB |
| `reveal_mode` | `immediate` or `tap` |
| `expires_at` | Nullable UTC timestamp |
| `max_views` | Nullable positive integer |
| `view_count` | Integer, default 0 |
| `created_at`, `updated_at` | UTC timestamps |

### `settings`

Key-value table. One key at launch: `signups_enabled`, default `false`.

## 5. Behaviour

### Pastes

- **Slug:** generated from `crypto/rand`; regenerated on the rare collision. The slug never changes on edit.
- **Signal colour:** one of four, chosen by a hash of the slug modulo 4, so each paste keeps its colour.
- **Sanitise on write:** the body is sanitised and normalised when saved. The read path is one row lookup and one template render.
- **Expiry choices:** 1 day, 1 week, 1 month, never. Editing a paste may change the expiry; the new expiry counts from the time of the edit.
- **View limit:** blank means unlimited. Editing does not reset the count.

### Reading

- A view is counted when the read page is fetched with `GET`, not when the reader taps reveal.
- Not counted: requests from known link-preview crawlers (matched by user agent: X/Twitterbot, Slack, Discord, Facebook, LinkedIn, Telegram, WhatsApp, Mastodon and similar), `HEAD` requests, and requests from the signed-in owner of the paste.
- Counting is atomic: a single `UPDATE pastes SET view_count = view_count + 1 WHERE slug = ? AND (max_views IS NULL OR view_count < max_views)`. If no row is updated on a limited paste, the reader gets the gone page. Two simultaneous readers cannot both take the last view.
- When the count reaches the limit, the reader who took the last view sees the paste and the row is deleted afterwards.
- Preview crawlers receive a page containing the title and preview metadata only, with no body.
- Preview metadata: `og:title` is the paste title; `og:description` is a fixed, generic line; no body text appears in any metadata.

### Gone

Expired, burned, deleted, never-existed and owner-disabled pastes all return the same gone page with HTTP status 404, so a link reveals nothing about what was there. A background sweep deletes expired pastes and expired sessions hourly; expiry is also checked at read time.

### Accounts

- **First admin:** created from `PASTR_ADMIN_USERNAME` and `PASTR_ADMIN_PASSWORD` at start-up when the users table is empty. If the table is empty and they are unset, the server refuses to start with a clear message.
- **Sign-up:** `/signup` returns 404 while sign-ups are disabled.
- **Disabling a user:** blocks sign-in, deletes their sessions, and makes their pastes return the gone page until re-enabled.
- **Password reset:** the admin sets a new password for a user. Users change their own password, display name and link on the account page.

## 6. Routes

| Surface | Routes | Access |
|---|---|---|
| Public read | `GET /p/{slug}` | Anyone |
| Account entry | `GET/POST /login`, `GET/POST /signup` | Anyone |
| Session | `POST /logout` | Signed in |
| Author | `GET /` (my pastes), `GET/POST /new`, `GET /p/{slug}/ready`, `GET/POST /p/{slug}/edit`, `POST /p/{slug}/delete` | Signed in; owner only for paste routes |
| Account | `GET/POST /account` | Signed in |
| Admin | `GET/POST /admin` | Admin only |
| Ops | `GET /healthz` | Anyone |
| Assets | `GET /static/*` | Anyone; long-lived cache with content-hashed names |

`GET /` redirects to `/login` when signed out. The ownership check lives in one middleware that every paste-management route passes through.

## 7. Security and privacy

### Authentication

- **Passwords:** Argon2id; minimum 10 characters; no composition rules.
- **Sessions:** 32 random bytes in a cookie that is `HttpOnly`, `Secure`, `SameSite=Lax`; valid 30 days; a new session is issued on every sign-in.
- **CSRF:** a per-session token in every state-changing form, plus an `Origin` header check on every non-`GET` request.
- **Throttling:** 5 failed sign-ins per username and per IP within 15 minutes triggers a temporary lockout. Sign-up is throttled per IP. Unknown usernames and wrong passwords return the same response and take the same time.

### Content

- **Sanitiser allowlist:** `div`, `p`, `br`, `strong`, `b`, `em`, `i`, `del`, `s`, `a[href]`, `h2`, `blockquote`, `ul`, `ol`, `li`. Everything else is stripped, including all other attributes.
- **Normalisation:** `h1` from the editor becomes `h2`, because the paste title is the page's `h1`.
- **Links:** `http`, `https` and `mailto` only; rendered with `rel="noopener noreferrer nofollow"`.
- **Limits:** 100 KB body, 120-character title, request size capped at the server.

### Headers

- **Content Security Policy:** `default-src 'none'`; styles and scripts from the site itself; scripts and connections additionally from the configured Umami origin; no inline script. Public pages therefore cannot run a script even if one survived sanitising.
- `X-Content-Type-Options: nosniff`, `frame-ancestors 'none'`, `Referrer-Policy: no-referrer` on paste pages, `X-Robots-Tag: noindex, nofollow` on paste pages.

### Privacy

- pastr stores no IP addresses and no reader data. View counts are bare integers.
- Throttling keeps IPs in memory only.
- The client IP is taken from `X-Forwarded-For` only when the request comes from the address in `PASTR_TRUSTED_PROXY`.

## 8. UX and visual design

Direction chosen through Impeccable: **Poster Wall**, after Swiss International Style concert posters. Every paste is a poster in a series. The whole identity is type, a grid and one coloured bar, so it needs no images, no web fonts and no JavaScript.

### Visual system

- **Type:** system grotesque stack (Helvetica Neue, Helvetica, Arial, system UI), flush left, ragged right, sizes stepped on one modular scale. Body text is 18px on phones at a 60 to 70 character measure.
- **Colour:** poster white ground, black ink, one signal colour per paste from a set of four (red, blue, green, orange). Dark theme: poster black ground, white ink, the same four hues adjusted for contrast.
- **Imagery:** one geometric bar per page, drawn in CSS.
- **Themes:** follow `prefers-color-scheme`; no toggle.
- **Monospace is not used anywhere.**

### Surfaces

| Surface | Layout |
|---|---|
| Read page | Title at poster size, the bar beneath it, one small meta line (byline if set, date, time left if expiring), then the text. No navigation, logo or footer. |
| Gated read page | First screen holds only the title, the bar and one typographic control, "Show the text". Built on `<details>`, so it works without JavaScript. |
| Gone page | "This one's gone." with the bar struck through the title position. |
| Login, sign-up | The form as the poster: bordered inputs, one button, the living poster composition beside or behind. |
| My pastes | A typographic index: title, views (and limit), time left, copy-link control. "New paste" is the only prominent action. The empty state shows the living poster and one line of guidance. |
| Create, edit | Title field at poster size with the hint "Shown in the link preview. Keep it spoiler-free." Then the editor, then one row of settings: reveal mode (two choices), expiry (four choices), view limit (blank means unlimited). |
| Link ready | Shown after saving: the URL and a copy button. |
| Account | Display name, link, change password. |
| Admin | Sign-up switch; users table with disable, enable and reset-password actions. |

### Motion

Read page, CSS only:

- **Bar draw:** the bar draws left to right as the page opens.
- **Title rise:** each title line rises from behind a mask, staggered; complete within about 0.3 seconds.
- **Reveal wipe:** tapping "Show the text" sweeps the signal colour across the screen; the text is present when it clears, within about 0.4 seconds.
- **Gone:** the bar draws in, then strikes through the title position.

Signed-in pages:

- **Page morph:** cross-document view transitions carry a paste's title from the list into the editor headline, and move the bar.
- **Link ready:** the URL sets itself character by character; "Link copied" flashes the bar.
- **Living poster:** a slow composition of bars and arcs in the four signal colours on login, sign-up and empty states. Never on the read page.

Rules:

- Body text is never animated and is visible at first paint.
- Only transform and opacity are animated.
- `prefers-reduced-motion` disables all of it.
- Browsers lacking view transitions or animated `<details>` get instant changes.

### Editor

Trix, pinned and vendored as one script and one stylesheet, loaded only on create and edit pages. File attachments and the code-block button are disabled. Toolbar: bold, italic, strikethrough, link, heading, quote, bulleted list, numbered list, undo, redo.

### Copy

Plain and friendly, never system language: "Show the text", "Link copied", "This one's gone", "That username or password isn't right".

### Accessibility

WCAG 2.2 AA. Signal colours meet contrast in both themes and never carry meaning alone. Every control is keyboard-reachable with visible focus. Pages hold at 200% zoom. The read page works with JavaScript disabled.

## 9. Analytics

The Umami tracker loads on every page when `PASTR_UMAMI_SCRIPT_URL` and `PASTR_UMAMI_WEBSITE_ID` are set; otherwise no analytics code is served. Events carry the paste slug where one applies.

| Event | Fired when | Mechanism |
|---|---|---|
| Page view | Any page opens | Umami automatic |
| `reveal` | Reader taps "Show the text" | `data-umami-event` attribute |
| `outbound-link` | Reader follows a link in a paste | Attribute added at render time |
| `gone` | An expired, burned or unknown link is opened | Small first-party script on the gone page |
| `paste-create`, `paste-edit`, `paste-delete` | Author saves or deletes; properties: reveal mode, expiry choice, whether a view limit is set | First-party script on signed-in pages |
| `link-copy` | Author copies a paste URL | First-party script |
| `login`, `signup` | Successful sign-in or registration | First-party script |

The read page ships no script other than the Umami tracker. Readers who block trackers do not appear in Umami, so its counts run lower than pastr's own view counts.

## 10. Configuration

| Variable | Purpose | Default |
|---|---|---|
| `PASTR_BASE_URL` | Public address for links and preview cards | required |
| `PASTR_ADDR` | Listen address | `:8080` |
| `PASTR_DB_PATH` | Database file | `/data/pastr.db` |
| `PASTR_ADMIN_USERNAME`, `PASTR_ADMIN_PASSWORD` | First admin, used only when no users exist | none |
| `PASTR_TRUSTED_PROXY` | Proxy address whose forwarded-IP header is trusted | none |
| `PASTR_UMAMI_SCRIPT_URL`, `PASTR_UMAMI_WEBSITE_ID` | Analytics | none |

## 11. Deployment

- **Image:** multi-stage Dockerfile; static binary with CGO disabled on a distroless base; runs as a non-root user.
- **Compose:** `docker-compose.yml` with one service and one named volume mounted at `/data`.
- **Health check:** `pastr healthcheck` subcommand calling `/healthz`, since the image has no shell.
- **TLS:** provided by the operator's reverse proxy; the README shows a Caddy example.
- **Backups:** the README documents a consistent copy of the SQLite file.
- **Shutdown:** the server drains in-flight requests on `SIGTERM`.

## 12. Error handling

- Form errors re-render the form with the entered values and a plain-language message beside the field.
- Unknown routes and unavailable pastes return the gone page with 404.
- Unauthorised access to another user's paste returns the same 404, not 403.
- Unexpected errors return a minimal 500 page in the same visual system and are logged with a request ID; no internal detail reaches the browser.
- Logs are structured (`log/slog`) and never contain passwords, session tokens or paste bodies.

## 13. Testing

- **Unit tests** per package. The sanitiser has table tests against a corpus of known XSS payloads.
- **Handler tests** using `httptest` and a temporary SQLite file, covering: sign-in and throttling, session expiry, CSRF and origin checks, ownership, sign-up toggle, expiry, view-limit races under concurrency, crawler handling, uniform gone responses, and security headers.
- **Manual browser checks** at phone and desktop sizes in light and dark themes during the build.
- **CI (GitHub Actions)** on every push: format check, `go vet`, `staticcheck`, `govulncheck`, `go test -race`, Docker build.

## 14. Repository

- **Licence:** MIT.
- **Included:** `README.md`, `LICENSE`, `SECURITY.md`, `.env.example`, `Makefile`, `Dockerfile`, `docker-compose.yml`, `.dockerignore`, `CLAUDE.md`, `PRODUCT.md`, `DESIGN.md`, and the design spec and implementation plan under `docs/`.
- **Ignored:** database files, `.env`, build output, editor folders, `.impeccable/`. Scratch files are kept outside the repository.
- **Commits:** small, conventional-style, on feature branches.

## 15. Reviews

Octopus code review and security audit at three checkpoints:

1. After storage and authentication.
2. After the paste read path, sanitiser and view limits.
3. Before the first public release, across the whole codebase.

Impeccable governs UI work: the direction contract is recorded before UI code, and the build ends with its finish review and a written `DESIGN.md`.

Creating the GitHub repository and pushing are done only with the owner's explicit go-ahead.

## 16. Changes made during the build

The reviews in section 15 led to these differences from the text above. Where they conflict, this section is what the code does.

- **Counting a view (section 5):** one transaction counts the view, re-checks expiry, limit and owner, and deletes a paste that has just used its last view. The hourly sweep also removes pastes that are at their limit.
- **Automated fetchers (section 5):** besides known preview crawlers, requests with no user agent, HTTP libraries and anything naming itself a bot get the title only and are not counted. Browser prefetches are refused, so the real visit fetches the page afresh.
- **Sign-in limits (section 7):** 5 tries per username from one address, 20 per address and 100 per username in 15 minutes, so a stranger's guesses cannot lock an owner out. IPv6 addresses are grouped by /64.
- **Passwords (sections 5 and 7):** changing or resetting a password ends the user's other sessions.
- **Trusted proxy (sections 7 and 10):** `PASTR_TRUSTED_PROXY` accepts an IP address or a CIDR range; the last `X-Forwarded-For` entry is used.
- **View limit on edit (section 5):** a limit at or below the views already used is refused.
- **Editor pages (section 7):** the create and edit pages add a per-response style nonce to the Content Security Policy for the styles Trix injects. No other page does.
- **Body size (section 7):** the 100 KB limit applies to the sanitised body as stored. The request cap is 512 KB, because a 100 KB body can triple when form-encoded.
- **Editor toolbar (section 8):** the nesting buttons are removed along with attachments and code blocks.
- **Motion (section 8):** the title rises as one block, not line by line. The reveal uses `visibility` and the title `clip-path` in addition to transform and opacity.
- **Navigation (section 8):** labels are "Pastes" and "New" so the bar fits one row on a phone.
- **Linting (section 13):** staticcheck runs from a pinned main-branch build until a release supports the Go version in use.

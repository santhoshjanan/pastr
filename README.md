# pastr

A small, self-hosted pastebin for prose and spoilers. Write the thing you can't say in the post, share the link, and the plot twist stays behind it.

- **Spoiler-safe previews:** link cards show only the title you wrote, never the text.
- **Tap to reveal:** choose per paste whether the text shows at once or after a tap.
- **Expiry and view limits:** a paste can disappear after a day, a week, a month, or a set number of views.
- **Fast to read:** the read page is server-rendered HTML with no JavaScript of its own, no web fonts and no images.
- **Private by default:** pastes are unlisted, and pastr stores no reader data.
- **Easy to run:** one container, one SQLite file.

## Quick start

```bash
cp .env.example .env
```

Edit `.env`: set `PASTR_BASE_URL` to the public address and choose an admin username and password (at least 10 characters). Then:

```bash
docker compose up -d
```

Sign in at your base URL. The admin variables are only read while no users exist, so you can remove them from `.env` afterwards.

Sign-ups are closed by default. The admin page has a switch to open them; people then register at `/signup`.

## Configuration

| Variable | Purpose | Default |
|---|---|---|
| `PASTR_BASE_URL` | Public address, used for links and preview cards. No trailing slash, no path. | required |
| `PASTR_ADMIN_USERNAME`, `PASTR_ADMIN_PASSWORD` | First admin account, created only when no users exist. | none |
| `PASTR_TRUSTED_PROXY` | Your reverse proxy as pastr sees it: an IP address or CIDR range. | none |
| `PASTR_UMAMI_SCRIPT_URL`, `PASTR_UMAMI_WEBSITE_ID` | Umami analytics. Set both or neither. | none |
| `PASTR_ADDR` | Listen address. | `:8080` |
| `PASTR_DB_PATH` | Database file. | `/data/pastr.db` |

## Running behind a reverse proxy

pastr speaks plain HTTP and expects a reverse proxy to provide TLS. With Caddy:

```
paste.example.com {
    reverse_proxy 127.0.0.1:8080
}
```

Set `PASTR_TRUSTED_PROXY` to the address (or Docker network range) the proxy connects from. pastr then reads the visitor's address from the last entry of `X-Forwarded-For` for sign-in throttling. The proxy must set or append that header itself, as Caddy, nginx (`proxy_add_x_forwarded_for`) and Traefik do; a proxy that passes a client's header through untouched would let visitors forge their address. Without it, everyone behind the proxy counts as one address and shares its sign-in limit.

Use an `https://` base URL in production: session cookies are marked `Secure` only then.

## Analytics

Point pastr at an Umami instance you run and it loads the tracker on every page. Leave the two variables unset and no analytics code is served.

| Event | Fired when |
|---|---|
| Page view | Any page opens |
| `reveal` | A reader taps "Show the text" (carries the paste slug) |
| `outbound-link` | A reader follows a link inside a paste |
| `gone` | Someone opens an expired, used-up or unknown link |
| `paste-create`, `paste-edit`, `paste-delete` | An author saves or deletes a paste |
| `link-copy` | An author copies a paste link |
| `login`, `signup` | A successful sign-in or registration |

The Umami script runs on the read page, so the Umami server sees paste URLs and could read page content. Only use an instance you control. Readers who block trackers do not appear in Umami, so its counts run lower than pastr's own view counts.

## How views are counted

A view is counted when a person opens the page, not when they tap reveal. Link-preview crawlers, search bots, HTTP libraries, browser prefetches and requests with no user agent get the title only and are not counted, so a chat app's preview cannot use up a one-view paste. Your own views of your own pastes are not counted either.

This relies on the visitor's user agent. A scanner that presents itself as an ordinary browser is counted as a reader.

## Backups

Everything lives in one SQLite file, `pastr.db`, on the `pastr-data` volume. Find where Docker keeps it:

```bash
docker volume inspect --format '{{ .Mountpoint }}' pastr_pastr-data
```

For a consistent copy while pastr is running, use SQLite's own backup on that file:

```bash
sqlite3 /path/to/pastr.db "VACUUM INTO '/path/to/backup.db'"
```

Or stop the container and copy the volume. If you would rather have the file in a folder you choose, replace the named volume in `docker-compose.yml` with a bind mount.

## Privacy and security

- pastr stores usernames, password hashes (Argon2id), session token hashes, and pastes. It stores no IP addresses and nothing about readers; a view count is a bare number.
- Paste text is reduced to a small set of formatting tags when saved. A strict Content Security Policy allows no inline scripts anywhere.
- Expired, used-up, deleted and unknown links all return the same page, so a link reveals nothing about what was there.
- There is no email, so there is no self-service password reset. The admin sets a new password for a user from the admin page.

Sign-in is limited to 5 tries per username from one address, 20 per address and 100 per username in 15 minutes.

To report a vulnerability, see [SECURITY.md](SECURITY.md).

## Development

Go is the only toolchain; there is no JavaScript build step.

```bash
make test
```

```bash
make run
```

`make run` serves http://localhost:8080 (this machine only) with a database in `./data` and the account `admin` / `change-me-please`. `make lint` runs the format check, `go vet`, staticcheck and govulncheck. `make docker` builds the image.

The design spec and implementation plan are in [docs/superpowers](docs/superpowers), the product record in [PRODUCT.md](PRODUCT.md) and the visual system in [DESIGN.md](DESIGN.md).

## Licence

[MIT](LICENSE). The bundled [Trix](https://trix-editor.org) editor is MIT-licensed by 37signals; see [internal/web/static/vendor/README.md](internal/web/static/vendor/README.md).

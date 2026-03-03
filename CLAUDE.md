# GovScout

## Overview

Web app that syncs federal contract opportunities from the SAM.gov API v2 and serves them via HTML templates + HTMX. Single binary, multi-user auth, saved search alerts. API key stored in `.env` as `SAMGOV_API_KEY`.

## Tech Stack

- Language: Go 1.23+
- Router: chi/v5
- Database: SQLite via modernc.org/sqlite (pure Go, no CGO)
- Templates: html/template + HTMX
- Sessions: gorilla/securecookie
- Passwords: golang.org/x/crypto/bcrypt
- CSS: minimal plain CSS (no framework)

## Architecture

```
cmd/govscout/main.go              # CLI: serve | sync | useradd
internal/
├── db/
│   ├── db.go                     # Open (DSN pragmas, WAL), migrate
│   ├── migrations/001_initial.sql # Full schema (go:embed)
│   ├── opportunities.go          # QueryBuilder, upsert, list, detail, stats
│   ├── users.go                  # User CRUD (bcrypt hashes)
│   ├── searches.go               # SavedSearch CRUD
│   ├── filters.go                # SavedFilter CRUD + seed defaults
│   ├── alerts.go                 # Alert insert (dedupe), delivery tracking
│   └── sync.go                   # sync_runs + backfill cursor (sync_state KV)
├── samgov/
│   ├── client.go                 # HTTP client, API key rotation (atomic), SearchWindow
│   └── types.go                  # SAM.gov API response structs
├── sync/
│   └── sync.go                   # Two-phase: incremental (3d) + backfill (90d windows)
├── alerts/
│   ├── matcher.go                # Keyword matching + alert delivery
│   └── email.go                  # Resend email delivery (rate-limited 1/day/search)
└── web/
    ├── server.go                 # Chi router, middleware stack
    ├── handlers.go               # All HTTP handlers
    ├── templates.go              # go:embed template loading + funcMap
    ├── auth.go                   # securecookie sessions, RequireAuth/RequireAdmin middleware
    ├── static/style.css          # Minimal CSS (embedded)
    └── templates/                # All HTML templates (embedded)
        ├── layout.html
        ├── login.html
        ├── opportunities.html
        ├── opportunity.html
        ├── partials/results.html
        ├── partials/pagination.html
        ├── alerts/list.html
        ├── alerts/detail.html
        ├── alerts/form.html
        ├── filters/list.html
        ├── filters/edit.html
        ├── admin/sync.html
        └── admin/users.html
```

## Build & Run

```bash
go build ./cmd/govscout                        # Build binary
./govscout serve                               # Start web server on :8080
./govscout sync                                # Daily sync (incremental + backfill)
./govscout sync --dry-run                      # Preview what would be fetched
./govscout sync --max-calls 5                  # Limit API calls for this run
./govscout sync --from 01/01/2015              # Backfill toward a specific date
./govscout useradd --username admin --password secret --admin  # Create admin user
./govscout passwd --username admin --password newpass          # Update user password
```

## Routes

Public: `GET /login`, `POST /login`, `POST /logout`, `GET /static/*`, `GET /health`

Auth required:

- `GET /opportunities` — full page with sidebar filters + HTMX
- `GET /opportunities/partial` — HTMX partial (results fragment)
- `GET /opportunities/{id}` — detail view
- `GET /alerts` — saved search list + recent alerts
- `GET /alerts/new`, `POST /alerts` — create saved search
- `GET /alerts/{id}`, `POST /alerts/{id}` — view/update saved search
- `POST /alerts/{id}/toggle` — enable/disable
- `GET /alerts/{id}/preview` — preview matching opportunities
- `GET /filters` — list + create saved filters
- `POST /filters` — create new filter
- `GET /filters/{id}`, `POST /filters/{id}` — edit/update filter
- `POST /filters/{id}/delete` — delete filter

Admin:

- `POST /admin/sync` — trigger sync in background
- `GET /admin/sync-runs` — sync history
- `GET /admin/users`, `POST /admin/users`, `POST /admin/users/{id}/delete` — user management

## Lint & Format

```bash
gofmt -l .            # Check formatting
gofmt -w .            # Auto-format
go vet ./...          # Lint
```

## Testing

```bash
go test ./...         # Run all tests
go build ./cmd/govscout  # Must compile cleanly
go vet ./...          # Must pass
```

## Environment Variables

See `.env.example`:

- `SAMGOV_API_KEY` — SAM.gov API key (required for sync). Supports comma-separated keys for rotation
- `AUTH_SECRET` — Session cookie signing secret, 32+ random chars
- `GOVSCOUT_DB` — SQLite database path (default: `./govscout.db`)
- `PORT` — Web server port (default: `8080`)
- `RESEND_API_KEY` — Resend API key for email alert delivery (optional)
- `RESEND_FROM_EMAIL` — Sender address for alert emails (default: `GovScout <alerts@resend.dev>`)

## API Details

- Single endpoint: `GET https://api.sam.gov/opportunities/v2/search`
- Auth: `api_key` query parameter
- Date format: `MM/DD/YYYY`
- Key query params: `limit`, `offset`, `postedFrom`, `postedTo`, `title`, `ptype`, `ncode`, `state`, `typeOfSetAside`, `noticeid`
- **Rate limiting**: SAM.gov enforces aggressive rate limits (~20 API calls/day per key). Do NOT increase `--max-calls` above 18. Multiple comma-separated keys enable automatic rotation on 429/401/403 responses.

## Key Design Decisions

- Single Go binary serves both web UI and handles sync
- All API response fields are `*string` — API returns inconsistent fields
- `active` column stored as INTEGER (1/0), converted from API's "Yes"/"No" at upsert time
- `raw_json` column stores full API response for each opportunity
- DB defaults to `./govscout.db` (override with `GOVSCOUT_DB`)
- Sessions via securecookie (HttpOnly, SameSite=Lax, 24h max-age)
- HTMX for live filtering without full page reloads
- Saved searches with keyword matching run after each sync
- SQLite driver: `modernc.org/sqlite` (pure Go, CGO_ENABLED=0)
- Date comparison in SQL uses string manipulation (`substr`) to compare MM/DD/YYYY dates

## Deployment

### Systemd

```bash
# Copy files
cp systemd/govscout.service /etc/systemd/system/
cp systemd/govscout-sync.service /etc/systemd/system/
cp systemd/govscout-sync.timer /etc/systemd/system/

# Enable
systemctl enable --now govscout
systemctl enable --now govscout-sync.timer
```

- `govscout.service` — web server as long-running service
- `govscout-sync.service` — one-shot sync
- `govscout-sync.timer` — daily at 2am, Persistent=true

## Sync

The `sync` command is designed for daily cron/timer use:

- **Incremental**: fetches last 3 days of opportunities (~1 API call)
- **Backfill**: uses remaining budget for historical data in 90-day windows
- **Rate limit safe**: stops gracefully on 429, saves cursor, resumes next run
- **Alert matching**: runs after sync to find new matches for saved searches

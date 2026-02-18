# GovScout

## Overview

Rust CLI tool that queries the SAM.gov Opportunities API v2 to search and view federal contract opportunities. API key stored in `.env` as `SAMGOV_API_KEY`.

## Tech Stack

- Language: Rust (2021 edition)
- CLI framework: clap (derive)
- HTTP: reqwest (blocking)
- Serialization: serde / serde_json
- Output: tabled (table formatting)
- Dates: chrono
- Errors: anyhow
- Env: dotenvy
- API server: axum, tokio, tower-http (CORS)
- Frontend: Next.js (App Router), TypeScript, Tailwind CSS, shadcn/ui (neo-brutalism theme), bun

## Architecture

```
src/
├── main.rs      # CLI definition (clap derive), subcommand routing, date defaults
├── lib.rs       # Library crate re-exporting modules
├── api.rs       # SamGovClient, SearchParams, API response types (serde)
├── db.rs        # SQLite persistence (rusqlite), schema init, upserts
├── display.rs   # Output formatting (tabled tables, detail views, reference tables)
├── sync.rs      # Daily sync logic (incremental + backfill with date windows)
└── server.rs    # Axum REST API server (read-only SQLite access)

web/                 # Next.js frontend (bun)
├── app/
│   ├── layout.tsx
│   ├── page.tsx                    # Opportunities list + filters + pagination
│   ├── login/page.tsx              # Login page (client component)
│   ├── api/auth/login/route.ts     # POST login (sets JWT cookie)
│   ├── api/auth/logout/route.ts    # POST logout (clears cookie)
│   └── opportunities/[id]/page.tsx # Detail view
├── components/
│   ├── ui/                         # shadcn neo-brutalism components
│   ├── logout-button.tsx           # Sign out button (client component)
│   ├── opportunity-card.tsx
│   ├── opportunity-detail.tsx
│   ├── search-filters.tsx
│   └── pagination.tsx
├── middleware.ts                    # Auth gate — redirects unauthenticated to /login
└── lib/
    ├── api.ts                      # Typed fetch client
    ├── auth.ts                     # JWT session helpers (jose)
    └── types.ts                    # TS types matching Rust API DTOs
```

## Build & Run

```bash
cargo build
cargo run --bin govscout -- search                                    # Auto-paginate all results (saves to DB)
cargo run --bin govscout -- search --title "cloud" --naics 541512     # Filtered search, all pages
cargo run --bin govscout -- search --limit 5                          # Single-page, max 5 results
cargo run --bin govscout -- get <NOTICE_ID>
cargo run --bin govscout -- types
cargo run --bin govscout -- sync                              # Daily sync (incremental + backfill)
cargo run --bin govscout -- sync --dry-run                    # Preview what would be fetched
cargo run --bin govscout -- sync --max-calls 5               # Limit API calls for this run
cargo run --bin govscout -- sync --from 01/01/2015            # Backfill from today toward a specific date
```

## Web Development

```bash
# Terminal 1: Rust API server (port 3001)
cargo run --bin govscout-server

# Terminal 2: Next.js frontend (port 3000)
cd web && bun dev
```

API endpoints:

- `GET /api/opportunities` — paginated list with query filters (search, naics_code, opp_type, set_aside, state, department, active_only, limit, offset)
- `GET /api/opportunities/:id` — full detail + contacts
- `GET /api/stats` — filter options with counts
- `GET /health` — health check

## Lint & Format

```bash
cargo fmt --check    # Check formatting
cargo fmt            # Auto-format
cargo clippy -- -D warnings  # Lint
```

## Testing

```bash
cargo test                                     # Run all unit tests (40 tests)
cargo test --lib                               # Library unit tests (31 tests)
cargo test display::tests                      # display.rs tests only
cargo test api::tests                          # api.rs tests only
cargo test db::tests                           # db.rs tests only
cargo test sync::tests                         # sync.rs tests only
cargo test --bin govscout-server               # server.rs tests only (QueryBuilder)
```

Smoke test with:

```bash
cargo build                                    # Must compile cleanly
cargo run --bin govscout -- search                            # Auto-paginate all results
cargo run --bin govscout -- search --title "cloud" --limit 5  # Single-page filtered search
cargo run --bin govscout -- get <notice_id>                   # Detail view
cargo run --bin govscout -- types                             # Reference table
```

## Environment Variables

See `.env.example`:

- `SAMGOV_API_KEY` — SAM.gov API key (required for CLI)
- `GOVSCOUT_DB` — SQLite database path (default: `./govscout.db`)
- `PORT` — API server port (default: `3001`)
- `ADMIN_USERNAME` — Web login username (required for frontend auth)
- `ADMIN_PASSWORD` — Web login password (required for frontend auth)
- `AUTH_SECRET` — JWT signing secret, 32+ random chars (required for frontend auth)

## API Details

- Single endpoint: `GET https://api.sam.gov/opportunities/v2/search`
- Auth: `api_key` query parameter
- Date format: `MM/DD/YYYY`
- Key query params: `limit`, `offset`, `postedFrom`, `postedTo`, `title`, `ptype`, `ncode`, `state`, `typeOfSetAside`, `noticeid`
- **Rate limiting**: SAM.gov enforces aggressive rate limits (~20 API calls/day per key). This is a hard platform constraint — do NOT increase `--max-calls` above 18 or attempt to work around rate limits. The sync command is carefully budgeted to stay within these limits (1-2 calls for incremental sync, remainder for backfill). Exceeding the limit results in 429 responses and temporary lockout.

## Key Design Decisions

- Uses `reqwest::blocking` (not async) — simplicity for a CLI tool
- All response fields are `Option<T>` — API returns inconsistent fields
- `--json` flag on `get` command serializes raw API response
- Default date range: 30 days ago to today
- `search` auto-paginates all results by default (1000/page); `--limit N` for single-page
- DB defaults to `./govscout.db` in current directory (override with `GOVSCOUT_DB` env var)
- Frontend auth via JWT session cookie (`jose` library, edge-runtime compatible)
- Next.js middleware protects all routes; `/login` and `/api/auth/*` are public
- Backend port unexposed in Docker; only reachable through authenticated Next.js proxy

## Deployment / Cron

The `sync` command is designed for daily cron use:

- **Incremental**: fetches last 3 days of opportunities (~1 API call) to stay current
- **Backfill**: uses remaining API budget to fetch historical data in 90-day windows going backwards (~4 years/run)
- **Rate limit safe**: stops gracefully on 429, saves progress, resumes next run
- **Steady state**: ~1-2 API calls/day once backfill is complete

Example cron: `0 2 * * * cd /path/to/govscout && ./target/release/govscout sync >> /var/log/govscout-sync.log 2>&1`

## Docker

### Development (`docker-compose.yml`)

```bash
docker compose up                    # Start both services
docker compose down                  # Stop
```

- Backend: mounts `src/`, `Cargo.toml`, `Cargo.lock` into `rust:latest` and runs `cargo run`; rebuild manually with `docker compose restart backend` after code changes
- Frontend: mounts `web/` into `oven/bun:1` and runs `bun dev`; Next.js HMR works out of the box
- Cargo build cache and `node_modules` are persisted in named volumes
- SQLite DB uses `./govscout.db` from the project root (bind-mounted)
- `SAMGOV_API_KEY` is read from `.env` in the project root
- Auth env vars (`ADMIN_USERNAME`, `ADMIN_PASSWORD`, `AUTH_SECRET`) passed to frontend service

### Production (`docker-compose.prod.yml`)

```bash
docker compose -f docker-compose.prod.yml build   # Build images
docker compose -f docker-compose.prod.yml up -d    # Start in background
docker compose -f docker-compose.prod.yml down      # Stop
```

- Backend: multi-stage build (`docker/Dockerfile.backend`) — compiles release binary, runs in slim Debian image
- Frontend: multi-stage build (`docker/Dockerfile.frontend`) — builds Next.js standalone output, runs with `bun`
- No source mounts or build caches; fully self-contained images
- Frontend `depends_on` backend healthcheck before starting
- Both services restart automatically (`unless-stopped`)

See also: [AGENTS.md](AGENTS.md) for agent-specific guidance.

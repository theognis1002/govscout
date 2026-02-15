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

## Architecture

```
src/
├── main.rs      # CLI definition (clap derive), subcommand routing, date defaults
├── api.rs       # SamGovClient, SearchParams, API response types (serde)
├── db.rs        # SQLite persistence (rusqlite), schema init, upserts
└── display.rs   # Output formatting (tabled tables, detail views, reference tables)
```

## Build & Run
```bash
cargo build
cargo run -- search
cargo run -- search --title "cloud" --naics 541512 --limit 5
cargo run -- get <NOTICE_ID>
cargo run -- types
```

## Lint & Format
```bash
cargo fmt --check    # Check formatting
cargo fmt            # Auto-format
cargo clippy -- -D warnings  # Lint
```

## Testing
No unit tests yet. Smoke test with:
```bash
cargo build                                    # Must compile cleanly
cargo run -- search                            # Smoke test with defaults
cargo run -- search --title "cloud" --limit 5  # Filtered search
cargo run -- get <notice_id>                   # Detail view
cargo run -- types                             # Reference table
```

## Environment Variables
See `.env.example`:
- `SAMGOV_API_KEY` — SAM.gov API key (required)

## API Details
- Single endpoint: `GET https://api.sam.gov/opportunities/v2/search`
- Auth: `api_key` query parameter
- Date format: `MM/DD/YYYY`
- Key query params: `limit`, `offset`, `postedFrom`, `postedTo`, `title`, `ptype`, `ncode`, `state`, `typeOfSetAside`, `noticeid`

## Key Design Decisions
- Uses `reqwest::blocking` (not async) — simplicity for a CLI tool
- All response fields are `Option<T>` — API returns inconsistent fields
- `--json` flag on `get` command serializes raw API response
- Default date range: 30 days ago to today

See also: [AGENTS.md](AGENTS.md) for agent-specific guidance.

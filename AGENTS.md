# Agent Guidelines for GovScout

## Code Style
- Use `anyhow::Result` for all error handling — no custom error types
- All API response structs derive `Deserialize, Serialize` with `#[serde(rename_all = "camelCase")]`
- Keep all response fields as `Option<T>` since SAM.gov returns inconsistent data
- Use `reqwest::blocking` — this is a synchronous CLI, not async

## Testing
```bash
cargo build                                    # Must compile cleanly
cargo run -- search                            # Smoke test with defaults
cargo run -- search --title "cloud" --limit 5  # Filtered search
cargo run -- get <notice_id>                   # Detail view
cargo run -- types                             # Reference table
```

## File Responsibilities
- **main.rs**: Only CLI arg parsing and routing. No business logic.
- **api.rs**: HTTP client, query construction, all serde types. No formatting/display.
- **db.rs**: SQLite persistence. Schema init, upserts. No HTTP or display logic.
- **display.rs**: All terminal output. No HTTP calls.

## SAM.gov API Notes
- Base URL: `https://api.sam.gov/opportunities/v2/search`
- API key goes as `api_key` query parameter (not header)
- Date format is `MM/DD/YYYY`
- NAICS filter param is `ncode` (not `naics`)
- The `get` subcommand uses the same search endpoint with `noticeid` param
- Rate limits exist but are generous for CLI use

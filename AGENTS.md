# Agent Guidelines for GovScout

## Code Style
- Standard Go conventions: `gofmt`, `go vet`
- All SAM.gov API response fields are `*string` (nullable) — API returns inconsistent data
- Use `map[string]any` for API response deserialization (flexible schema)
- Error handling via standard `error` returns

## Testing
```bash
go build ./cmd/govscout          # Must compile cleanly
go vet ./...                     # Must pass
go test ./...                    # Run tests
./govscout useradd --username admin --password test --admin
./govscout serve                 # Start on :8080
./govscout sync --dry-run        # Preview sync
```

## File Responsibilities
- **cmd/govscout/main.go**: CLI arg parsing (serve/sync/useradd). No business logic.
- **internal/samgov/**: HTTP client, API key rotation, response types. No DB or display.
- **internal/db/**: SQLite persistence. Schema, upserts, queries. No HTTP.
- **internal/sync/**: Sync orchestration (incremental + backfill). Uses samgov + db.
- **internal/alerts/**: Keyword matching, webhook delivery. Runs after sync.
- **internal/web/**: HTTP server, handlers, templates, auth. Read-only DB access (except admin sync).

## SAM.gov API Notes
- Base URL: `https://api.sam.gov/opportunities/v2/search`
- API key goes as `api_key` query parameter (not header)
- Date format is `MM/DD/YYYY` (Go layout: `"01/02/2006"`)
- NAICS filter param is `ncode` (not `naics`)
- Rate limits: ~20 calls/day per key. Do not exceed --max-calls 18.
- Key rotation on 429/401/403 via atomic index

# Contributing to GovScout

## Development Setup

1. Install [Go](https://go.dev/dl/) 1.23+
2. Clone the repository and `cd` into it
3. Copy `.env.example` to `.env` and add your SAM.gov API key + auth secret
4. Build: `go build ./cmd/govscout`

## Running Locally

```bash
# Create an admin user
./govscout useradd --username admin --password secret --admin

# Start the web server
./govscout serve

# Run a sync (dry-run first)
./govscout sync --dry-run
./govscout sync --max-calls 5
```

## Git Hooks

Set up pre-commit hooks to catch formatting and lint issues before committing:

```bash
git config core.hooksPath .githooks
```

## Code Quality

Before submitting a PR, ensure your code passes:

```bash
gofmt -l .           # Check formatting (should produce no output)
go vet ./...         # Lint
go build ./cmd/govscout  # Must compile cleanly
go test ./...        # Run tests
```

## Submitting Changes

1. Fork the repository and create a feature branch
2. Make your changes with clear, focused commits
3. Ensure `gofmt`, `go vet`, and `go build` all pass
4. Open a pull request with a description of what changed and why

## Code Style

- Standard Go conventions: `gofmt`, `go vet`
- All SAM.gov API response fields are `*string` (API returns inconsistent data)
- Use `map[string]any` for API response deserialization (flexible schema)
- Error handling via standard `error` returns, no panics
- `cmd/govscout/main.go` handles CLI parsing only
- `internal/samgov/` handles HTTP; `internal/db/` handles persistence; `internal/web/` handles the web UI

## Questions?

Open an issue for bugs, feature requests, or questions.

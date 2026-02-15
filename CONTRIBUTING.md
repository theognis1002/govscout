# Contributing to GovScout

## Development Setup

1. Install [Rust](https://rustup.rs/) (stable toolchain)
2. Clone the repository and `cd` into it
3. Copy `.env.example` to `.env` and add your SAM.gov API key
4. Build: `cargo build`

## Running Locally

```bash
cargo run -- search --title "cloud" --limit 5
cargo run -- get <NOTICE_ID>
cargo run -- types
```

## Code Quality

Before submitting a PR, ensure your code passes:

```bash
cargo fmt --check
cargo clippy -- -D warnings
cargo build
```

If you've set up the git hooks, these run automatically on commit:

```bash
git config core.hooksPath .githooks
```

## Submitting Changes

1. Fork the repository and create a feature branch
2. Make your changes with clear, focused commits
3. Ensure `cargo fmt`, `cargo clippy`, and `cargo build` all pass
4. Open a pull request with a description of what changed and why

## Code Style

- Use `anyhow::Result` for error handling
- Keep API response fields as `Option<T>` (SAM.gov returns inconsistent data)
- `main.rs` handles CLI parsing only; `api.rs` handles HTTP; `display.rs` handles output

## Questions?

Open an issue for bugs, feature requests, or questions.

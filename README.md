# GovScout

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

CLI tool to search and view federal contract opportunities from the [SAM.gov](https://sam.gov) Opportunities API.

## Overview

GovScout lets you search, filter, and inspect federal contract opportunities directly from your terminal. It queries the SAM.gov Opportunities API v2, returning results as formatted tables or raw JSON. Useful for government contractors, business development teams, and anyone tracking federal procurement.

## Features

- Search opportunities by keyword, NAICS code, type, state, and set-aside
- View detailed opportunity information including contacts, awards, and descriptions
- Filter by date range with sensible defaults (last 30 days)
- Output as formatted tables or raw JSON
- Built-in reference tables for opportunity types and set-aside codes

## Prerequisites

- [Rust](https://rustup.rs/) (stable toolchain)
- A free SAM.gov API key ([register here](https://sam.gov/content/home))

## Quick Start

1. Clone the repository:
   ```bash
   git clone https://github.com/theognis1002/govscout.git
   cd govscout
   ```

2. Configure your API key:
   ```bash
   cp .env.example .env
   # Edit .env and add your SAM.gov API key
   ```

3. Build and run:
   ```bash
   cargo build --release
   ./target/release/govscout search
   ```

## Usage

```bash
# Search recent opportunities (last 30 days)
govscout search

# Search by keyword
govscout search --title "cloud migration" --limit 5

# Filter by NAICS code (software-related)
govscout search --naics 541512
govscout search --naics 541511 --title "cybersecurity"

# Filter by type and set-aside
govscout search --ptype k --set-aside SBA

# View a specific opportunity
govscout get <NOTICE_ID>

# View as raw JSON
govscout get <NOTICE_ID> --json

# Show reference codes
govscout types
```

### Example Searches for Software Contractors

```bash
# All active software-related solicitations
govscout search --naics 541512 --ptype k --limit 20

# Cloud computing opportunities (combined synopsis)
govscout search --title "cloud" --naics 541512 --ptype k

# Cybersecurity sources sought (market research stage)
govscout search --title "cybersecurity" --ptype r

# Small business set-aside IT contracts
govscout search --naics 541512 --set-aside SBA

# Award notices for competitive intelligence
govscout search --naics 541512 --ptype a --limit 20
```

## Architecture

```
src/
├── main.rs      # CLI definition (clap), subcommand routing, date defaults
├── api.rs       # HTTP client, query construction, API response types
└── display.rs   # Terminal output formatting (tables, detail views)
```

## Configuration

| Variable | Description |
|----------|-------------|
| `SAMGOV_API_KEY` | Your SAM.gov API key (required) |

See [.env.example](.env.example) for the template.

## Reference Tables

### Software-Related NAICS Codes

| Code | Description |
|------|-------------|
| **541511** | Custom Computer Programming Services |
| **541512** | Computer Systems Design Services |
| **541513** | Computer Facilities Management Services |
| **541519** | Other Computer Related Services |
| **541715** | R&D in Physical, Engineering, and Life Sciences |
| **518210** | Data Processing, Hosting, and Related Services |
| **511210** | Software Publishers |

### Opportunity Types

| Code | Type | Description |
|------|------|-------------|
| `o` | Solicitation | Active solicitation accepting bids/proposals |
| `p` | Presolicitation | Notice of upcoming solicitation |
| `k` | Combined Synopsis/Solicitation | Combined notice (common for simplified acquisitions) |
| `r` | Sources Sought | Market research; agency seeking capability statements |
| `s` | Special Notice | Informational (conferences, training, events) |
| `a` | Award Notice | Contract has been awarded |
| `u` | J&A | Justification for other-than-full-and-open competition |
| `g` | Intent to Bundle | Notice of intent to bundle requirements |
| `i` | Fair Opportunity | Justification for limiting competition |

### Set-Aside Codes

| Code | Description |
|------|-------------|
| `SBA` | Total Small Business Set-Aside |
| `SBP` | Partial Small Business Set-Aside |
| `8A` | 8(a) Set-Aside |
| `8AN` | 8(a) Sole Source |
| `HZC` | HUBZone Set-Aside |
| `HZS` | HUBZone Sole Source |
| `SDVOSBC` | Service-Disabled Veteran-Owned Small Business Set-Aside |
| `SDVOSBS` | Service-Disabled Veteran-Owned Small Business Sole Source |
| `WOSB` | Women-Owned Small Business Set-Aside |
| `WOSBSS` | Women-Owned Small Business Sole Source |
| `EDWOSB` | Economically Disadvantaged WOSB Set-Aside |
| `EDWOSBSS` | Economically Disadvantaged WOSB Sole Source |

## Development

```bash
cargo build              # Build
cargo fmt --check        # Check formatting
cargo clippy             # Lint
cargo run -- search      # Smoke test
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for full development guidelines.

## Contributing

Contributions are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md) for details.

## License

[MIT](LICENSE)

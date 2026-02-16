# GovScout Web Frontend

## Tech Stack
- Next.js 16 (App Router, React Server Components)
- TypeScript, Tailwind CSS v4
- shadcn/ui with neo-brutalism theme
- bun (package manager and runtime)

## Commands
```bash
bun dev          # Dev server on port 3000
bun run build    # Production build
bun run lint     # ESLint
```

## Architecture
- Server components fetch data from the Rust API (`lib/api.ts`)
- Client components handle interactivity (filters, pagination)
- API proxy: Next.js rewrites `/api/*` to `http://localhost:3001/api/*`
- Types in `lib/types.ts` mirror the Rust API response DTOs

## Key Patterns
- All API types use `string | null` (matching Rust `Option<String>`)
- Search filters are URL search params (shareable URLs, server-side fetching)
- `OpportunityCard` links to `/opportunities/[id]` detail page
- Neo-brutalism theme: 0px radius, bold shadows, DM Sans font

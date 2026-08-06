# nivek

A Go-based Twitch integration platform for live Dwarf Fortress streaming.

**nivek** lets Twitch chat interact with a running Dwarf Fortress game while automated snapshots feed a public web dashboard. The repository contains independent Go services, a Nuxt frontend, PostgreSQL persistence, EventSub integration, HMAC-authenticated machine-to-machine APIs, and an outbound snapshot pipeline.

## Architecture

![Architecture diagram](docs/architecture.png)

- **`cmd/twitch-bot`** — Twitch IRC bot that joins active channels, validates chat input, and forwards supported commands.
- **`cmd/df-executor`** — Receives authenticated commands and executes them on the Dwarf Fortress host.
- **`cmd/df-snapshot-pusher`** — Reads atomic DFHack snapshots, validates and compresses them, signs the payload, and sends it outbound to Core API.
- **`cmd/core-api`** — Echo backend for Twitch sign-in, user features, snapshot ingestion, and dashboard APIs.
- **`nivek-nuxt`** — Nuxt 4 SSR frontend and public dashboard.

## Authentication

Twitch OAuth is the only user sign-in path. The local session is stored in an HttpOnly cookie, not in localStorage or a URL. Unsafe cookie-authenticated requests require a CSRF header. See [Authentication architecture](docs/AUTHENTICATION.md) for the complete flow and deployment requirements.

The production callback must be registered exactly as:

```text
https://peanutbudderbot.com/api/auth/twitch/callback
```

## Setup

```bash
cp .env.example .env
# Fill every required secret and Twitch application value.
docker compose up --build
```

Generate secrets rather than inventing memorable strings:

```bash
openssl rand -base64 48   # JWT secret
openssl rand -hex 32      # HMAC keys
```

## Validation

```bash
go test -race ./...
go vet ./...

cd nivek-nuxt
npm ci
npm run build
```

## Security properties

- OAuth state cookie with constant-time comparison and one-time deletion
- HttpOnly local session cookie plus double-submit CSRF defense
- HS256 allow-listing with issuer, audience, subject, JTI, time claims, and a minimum 32-byte signing secret
- Strict configuration validation before the HTTP server starts
- Narrow credentialed CORS policy derived from the frontend origin
- Request recovery, body limits, security headers, and HTTP server timeouts
- HMAC-authenticated internal APIs
- PostgreSQL isolated on an internal Docker network
- Read-only, capability-dropped application containers
- Pull-request CI for Go tests, race detection, vetting, formatting, and Nuxt production builds

See [SECURITY.md](SECURITY.md) for responsible disclosure and operational guidance.

**Stack:** Go, Echo, PostgreSQL, Nuxt, Twitch OAuth/Helix/EventSub, WebSockets, HMAC-SHA256, JWT, Docker, Traefik.

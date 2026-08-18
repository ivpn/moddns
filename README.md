# modDNS · Secure DNS System

modDNS is a full-stack DNS security platform that combines encrypted DNS transport, fine-grained content filtering, and rich analytics. The project is maintained by IVPN and released under the GPL-3.0 license.

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [Core Technologies](#core-technologies)
3. [Features](#features)
4. [Getting Started](#getting-started)
5. [Repository Layout](#repository-layout)
6. [Development Workflow](#development-workflow)
7. [Testing](#testing)
8. [Contributing](#contributing)
9. [Code of Conduct & Security](#code-of-conduct--security)
10. [License](#license)
11. [Community & Support](#community--support)

## Architecture Overview
modDNS is built as a microservices architecture with the following components:

```
┌──────────────┐   ┌──────────────┐   ┌──────────────┐
│  Web Client  │──▶│    Nginx     │──▶│   Frontend   │
│  (browser)   │   │ (:80, static)│   │ (React SPA)  │
└──────┬───────┘   └──────────────┘   └──────────────┘
       │ REST API (:3000)
       ▼
┌──────────────┐   ┌──────────────────────┐
│  API Server  │──▶│        Redis         │
│  (Go/Fiber)  │   │ (master + 2 replicas │
└──────┬───────┘   │     + sentinel)      │
       │           └──────────▲───────────┘
       ▼                      │ profile/rule reads
┌──────────────┐              │ (dns replica)
│   MongoDB    │   ┌──────────┴───┐   ┌──────────────────┐
│              │◀──│  DNS Proxy   │──▶│    Recursors     │
└──────────────┘   │  (TLS term.) │   │  (sdns + Knot)   │
   query logs &    └──────▲───────┘   └──────────────────┘
   statistics             │ DoH / DoT / DoQ
                   ┌──────┴───────┐
                   │ DNS Clients  │
                   └──────────────┘
```

Additional services not shown above:

- **blocklists** — periodically downloads and ingests filter lists (AdGuard, Hagezi, OISD, StevenBlack, …) into Redis/MongoDB.
- **dnscheck** — standalone DNS diagnostics microservice with GeoIP (MaxMind) lookup.

Note that the DNS proxy terminates TLS for the encrypted DNS transports itself; Nginx only serves the web frontend. The API is exposed directly on port 3000.

## Core Technologies

**Backend Services**
- Go & Fiber for high-performance APIs
- MongoDB for persistent storage of accounts, profiles, query logs, and statistics
- Redis (master + two replicas + sentinel) for caching and distributing profile configuration to the proxy
- sdns and Knot Resolver as bundled recursors (Knot is the default)

**Frontend**
- React + TypeScript SPA (shadcn/ui & Radix UI component system)
- Tailwind CSS for utility-first styling
- PWA with offline support and in-app update flow

**Infrastructure**
- Docker & Docker Compose for local orchestration
- Nginx serving the web frontend (the DNS proxy terminates TLS for encrypted DNS transports itself)
- GitHub Actions for CI/CD automation

## Features

**Encrypted DNS**
- DNS over HTTPS (DoH), DNS over TLS (DoT), DNS over QUIC (DoQ)
- Per-profile DNS stamps (`sdns://`) calculator and dnscrypt-proxy (via DoH) setup support
- DNSSEC validation (per profile, enabled by default)

**Content Filtering**
- Built-in blocklists (ads, malware, trackers)
- Custom allow/deny rules per profile — domains and IPs, with rule groups and precedence
- Service-based blocking presets backed by ASN lookup (Google, Meta, TikTok, Netflix, …)
- DNS rebinding protection

**User & Profile Management**
- Multi-profile accounts with individualized policies
- MFA (TOTP and WebAuthn/passkeys), email verification, and secure password workflows
- Profile settings export & import
- In-app announcements

**Observability**
- Near real-time DNS query logging with outcome classification and quick-rule creation from log entries (opt-in, configurable retention from 1 hour to 1 month)
- Statistics and exportable analytics for auditing
- Prometheus metrics exposed by the proxy

**Apple Device Integration**
- Managed `.mobileconfig` profiles
- QR-code assisted enrollment for mobile clients

## Getting Started

### Prerequisites
- Docker & Docker Compose
- Make (for the provided automation scripts)
- Node.js 22+ and npm (for the React application)
- Go 1.26.5+ for `proxy/`, Go 1.25.8+ for the other Go modules (toolchain versions pinned in the `go.mod` files)
- Python 3.11 (for the backend E2E tests)
- mkcert (optional, for trusted local TLS certificates)

### Quick Start

Before the first `make up`, create the required (gitignored) environment files from the tracked samples and provide the GeoLite2 databases:

```bash
# 1. Environment files
cp api/.env.sample api/.env
cp proxy/.env.sample proxy/.env
cp dnscheck/.env.sample dnscheck/.env

# 2. MaxMind GeoLite2 databases (mounted by the proxy and dnscheck)
#    Place them under dev/bootstrap/GeoLite2-ASN/ and dev/bootstrap/GeoLite2-City/
```

Then:

```bash
make up          # Build and start every service stack
make down        # Stop and remove containers
```

Certificates for local HTTPS access live in `dev/certs/`. You can either generate them with `mkcert` or import the bundled CA into your OS trust store.

## Repository Layout

| Path      | Purpose |
|-----------|---------|
| `api/`    | Go API service (REST + DNS logic)
| `app/`    | React front-end (shadcn/ui + Tailwind)
| `blocklists/` | Blocklist ingestion and update tooling
| `dnscheck/` | DNS diagnostics microservice
| `libs/`   | Shared Go libraries (logging, cache, store)
| `proxy/`  | DNS proxy implementation
| `tests/`  | Integration and regression suites (pytest + testcontainers)
| `dev/bootstrap/`, `compose.*.yml` | Docker-compose orchestration and bootstrap assets
| `dev/certs/`  | Development TLS certificates and local CA
| `scripts/` | Helper scripts
| `.github/` | CI workflows (GitHub Actions), lint configs, issue/PR templates

## Development Workflow

### Frontend (`app/`)
```bash
cd app
npm install
npm run dev       # Launches Vite dev server on http://localhost:5173
npm run lint
npm run tsc
npm run build
```
`npm run dev` sources `app/env/.env.local`, which is gitignored — create it first (see the tracked `app/env/.env.production`, `.env.staging`, and `.env.test` for reference).

### API service (`api/`)
```bash
cd api
go mod tidy
make test
```
(See `api/Makefile` for additional targets like `make lint`, `make gow` (live reload), `make swag`, and `make mockery`. From the repo root, `make dev_api` runs live reload inside the running container.)

### Proxy service (`proxy/`)
```bash
cd proxy
go mod tidy
make test
```
(See `proxy/Makefile` for additional targets like `make lint`, `make gow`, and `make mockery`. From the repo root, `make dev_proxy` runs live reload inside the running container.)

### Backend E2E tests (`tests/`)
```bash
cd tests
python3.11 -m venv venv
source venv/bin/activate
make install_test_dependencies
make test_ci        # spins up the stack via testcontainers
make test_failover  # destructive Redis failover tests (excluded from test_ci, run separately)
```

## Testing

- **Web client**: `npm run lint && npm run tsc && npm run test` (unit) and `npm run test:e2e` (Playwright)
- **Go services**: `go test ./...` inside each Go module (`api/`, `blocklists/`, `proxy/`, etc.)
- **Integration**: `cd tests && source venv/bin/activate && make test_ci`
- **Static analysis**: `make lint` in relevant directories (Go linters + ESLint)

## Contributing

Please see [`CONTRIBUTING.md`](.github/CONTRIBUTING.md) for detailed guidelines on how to propose changes, open issues, and submit pull requests.

## Code of Conduct & Security

Please refer to [`CODE_OF_CONDUCT.md`](CODE_OF_CONDUCT.md) for expectations in all community spaces.

Security vulnerabilities should **not** be disclosed in public issues. Follow the process described in [`SECURITY.md`](.github/SECURITY.md) to report them responsibly.

## License

modDNS is licensed under the [GNU GPL-3.0](LICENSE.md). By contributing, you agree that your contributions will be licensed under the same terms.

## Community & Support

- GitHub Issues: bug reports, feature requests, documentation updates
- Discussions & PR reviews: architectural questions, roadmap proposals
- Commercial support: contact IVPN through the official support portal if you are an IVPN customer

We look forward to your contributions—whether code, documentation, or feedback.

<p align="center">
  <img alt="modDNS" src="app/src/assets/logos/modDNS-light-theme.svg#gh-light-mode-only" width="320">
  <img alt="modDNS" src="app/src/assets/logos/modDNS-dark-theme.svg#gh-dark-mode-only" width="320">
</p>

<p align="center">
  Privacy-first DNS filtering service with DoH, DoT and DoQ, blocklists, custom rules and per-profile settings. Independently audited. Built by IVPN.
</p>

<p align="center">
  <a href="https://moddns.net">modDNS website</a> ·
  <a href="https://www.ivpn.net">IVPN website</a> ·
  <a href="https://moddns.net/faq">FAQ</a> ·
  <a href="https://moddns.net/privacy">Privacy Policy</a>
</p>

<p align="center">
  <a href="LICENSE.md"><img alt="License: GPL-3.0" src="https://img.shields.io/github/license/ivpn/moddns"></a>
  <a href="https://github.com/ivpn/moddns/releases/latest"><img alt="Latest release" src="https://img.shields.io/github/v/release/ivpn/moddns"></a>
</p>

## Table of Contents

1. [About](#about)
2. [Features](#features)
3. [Architecture](#architecture)
4. [Repository layout](#repository-layout)
5. [Development](#development)
6. [Testing](#testing)
7. [Contributing](#contributing)
8. [Security](#security)
9. [License](#license)

## About

modDNS is a hosted DNS service that blocks ads, trackers and malware on every device, with encrypted transport and full control over what gets logged. It is developed and operated by IVPN.

modDNS is included in the [IVPN](https://www.ivpn.net/pricing/) Plus and Pro Suite plans. This repository contains the source code of the service. Self-hosting is not supported: the setup instructions below are for contributors running the stack locally.

## Features

**Encrypted DNS**
- DNS over HTTPS (DoH), DNS over TLS (DoT) and DNS over QUIC (DoQ)
- DNS stamps (`sdns://`) for each profile, for use with dnscrypt-proxy over DoH
- DNSSEC validation, on by default and configurable per profile

**Filtering**
- Curated blocklists for ads, trackers, malware and more (AdGuard, Hagezi, OISD, StevenBlack and others)
- Custom allow and block rules for domains and IPs, with rule groups and precedence
- Block whole services (Google, Meta, TikTok, Netflix and others) with one toggle
- DNS rebinding protection

**Accounts and profiles**
- Several profiles per account, each with its own settings
- Two-factor authentication with TOTP or passkeys (WebAuthn)
- Export and import of profile settings

**Logs and statistics**
- Query logs are off by default. When enabled, they show each query's outcome, let you create a rule from any entry and are kept from 1 hour to 1 month
- Opt-in statistics on queries and blocking over time

**Device setup**
- Setup guides for every major platform
- Apple configuration profiles (`.mobileconfig`) with QR-code enrollment on mobile

## Architecture

modDNS is a set of Go microservices with a React frontend:

<p align="center">
  <img alt="modDNS architecture: DNS clients connect to the DNS proxy over DoH, DoT and DoQ; the proxy forwards queries to Knot Resolver, reads settings from Redis and writes query logs and statistics to MongoDB. The web app talks to the API and dnscheck; the API stores accounts in MongoDB and publishes profile settings to Redis; blocklists loads filter lists into Redis and MongoDB." src=".github/assets/architecture-light.svg#gh-light-mode-only">
  <img alt="modDNS architecture: DNS clients connect to the DNS proxy over DoH, DoT and DoQ; the proxy forwards queries to Knot Resolver, reads settings from Redis and writes query logs and statistics to MongoDB. The web app talks to the API and dnscheck; the API stores accounts in MongoDB and publishes profile settings to Redis; blocklists loads filter lists into Redis and MongoDB." src=".github/assets/architecture-dark.svg#gh-dark-mode-only">
</p>

- The **DNS proxy** receives DoH, DoT and DoQ queries and applies each profile's filtering.
- The **API** (Go, Fiber, MongoDB) manages accounts and profiles, writes profile settings to Redis for the proxy to read, and serves query logs and statistics to the web app.
- **Knot Resolver** is the recursive resolver behind the proxy.
- **blocklists** downloads filter lists on a schedule and loads them into Redis and MongoDB.
- **dnscheck** is a DNS diagnostics service with GeoIP (MaxMind) lookup.
- The **frontend** is a React + TypeScript PWA built with shadcn/ui, Radix UI and Tailwind CSS.

## Repository layout

| Path | Purpose |
|------|---------|
| `api/` | API service (Go, Fiber, MongoDB) |
| `proxy/` | DNS proxy |
| `blocklists/` | Blocklist ingestion |
| `dnscheck/` | DNS diagnostics service |
| `libs/` | Shared Go libraries, vendored by `api/` and `proxy/` |
| `app/` | React frontend |
| `tests/` | Backend end-to-end tests (pytest + testcontainers) |
| `dev/`, `compose.*.yml` | Local Docker Compose stack, bootstrap data and development TLS certificates |
| `scripts/` | Helper scripts |
| `.github/` | CI workflows, lint configs, issue and PR templates |

## Development

### Prerequisites

- Docker and Docker Compose
- Make
- Go 1.26.8+ (pinned in each `go.mod`)
- Node.js 22+ and npm
- Python 3.11 (backend end-to-end tests)
- mkcert (optional, for trusted local TLS certificates)

### Running the stack locally

1. Create the environment files from the tracked samples:

   ```bash
   cp api/.env.sample api/.env
   cp proxy/.env.sample proxy/.env
   cp dnscheck/.env.sample dnscheck/.env
   ```

2. Download the free [MaxMind GeoLite2-ASN](https://dev.maxmind.com/geoip/geolite2-free-geolocation-data) database (requires a MaxMind account) and place `GeoLite2-ASN.mmdb` in `dev/bootstrap/GeoLite2-ASN/`. The proxy and dnscheck mount it.

3. Start and stop the stack:

   ```bash
   make up      # build and start all services
   make down    # stop and remove containers
   ```

   `make up_dev` starts a smaller stack (single Redis, Knot only). Run `make help` for all targets.

Development TLS certificates are in `dev/certs/`. Generate your own with mkcert or import the bundled CA into your OS trust store. [`README-dev.md`](README-dev.md) covers certificates, local DNS overrides and client configuration in detail, and [`README-QA.md`](README-QA.md) shows how to point browsers and devices at the local stack.

### Frontend (`app/`)

```bash
cd app
npm install
npm run dev     # Vite dev server on http://localhost:5173
```

`npm run dev` reads `app/env/.env.local`, which is gitignored. Create it based on the tracked `app/env/.env.production`, `.env.staging` or `.env.test`.

### Go services

Each Go module (`api/`, `proxy/`, `blocklists/`, `dnscheck/`, `libs/`) is tested on its own. In `api/` and `proxy/`, `make lint`, `make gow` (live reload) and `make mockery` are also available, plus `make swag` in `api/`. From the repository root, `make dev_api` and `make dev_proxy` run live reload inside the running containers.

`api/` and `proxy/` vendor `libs/`. After changing `libs/`, run `go mod vendor` in both.

## Testing

| Area | Command |
|------|---------|
| Frontend | `cd app && npm run lint && npx tsc -p tsconfig.app.json && npm run test` |
| Frontend E2E (Playwright) | `cd app && npm run test:e2e -- <area>` |
| API | `cd api && go test ./...` |
| Proxy | `cd proxy && go test -race ./...` |
| Other Go modules | `go test ./...` in `blocklists/`, `dnscheck/` or `libs/` |
| Backend E2E | see below |

Backend end-to-end tests start the stack with testcontainers:

```bash
cd tests
python3.11 -m venv venv
source venv/bin/activate
make install_test_dependencies
make test_ci         # main suite
make test_failover   # destructive Redis failover tests, run separately
```

## Contributing

See [`CONTRIBUTING.md`](.github/CONTRIBUTING.md) for how to open issues and submit pull requests. Contributions are accepted under the [Contributor License Agreement](CLA.md) and the [Code of Conduct](CODE_OF_CONDUCT.md) applies in all project spaces.

- **Bugs and feature requests:** [GitHub Issues](https://github.com/ivpn/moddns/issues)
- **Questions and ideas:** [GitHub Discussions](https://github.com/ivpn/moddns/discussions)
- **Account and service support:** [moddns@ivpn.net](mailto:moddns@ivpn.net)

## Security

Do not report vulnerabilities in public issues. Follow [`SECURITY.md`](.github/SECURITY.md) to report them privately.

## License

modDNS is licensed under the [GNU GPL-3.0](LICENSE.md).

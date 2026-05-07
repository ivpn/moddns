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
┌──────────────┐    ┌──────────────┐    ┌──────────────┐    ┌──────────────┐
│              │───▶│              │───▶│              │───▶│              │
│  Web Client  │    │ Nginx Proxy  │    │   Frontend   │    │ API Server   │
│              │    │              │    │   (React)    │    │              │
└──────────────┘    └──────────────┘    └──────────────┘    └──────┬───────┘
                                                                   │
                                                        ┌──────────┴──────────┐
                                                        │                     │
                                                        ▼                     ▼
                                                ┌──────────────┐      ┌──────────────┐
                                                │              │      │              │
                                                │    Redis     │      │   MongoDB    │
                                                │  (Caching)   │      │  (Storage)   │
                                                └──────────────┘      └──────────────┘
                                                        ▲
                                                        │
┌──────────────┐    ┌──────────────┐                    │
│              │───▶│              │────────────────────┘
│ DNS Clients  │    │  DNS Proxy   │
│              │    │              │
└──────────────┘    └──────┬───────┘
                           │
                           ▼
                    ┌──────────────┐
                    │              │
                    │ DNS Resolver │
                    │(SDNS/Unbound)│
                    └──────────────┘
```

## Core Technologies

**Backend Services**
- Go & Fiber for high-performance APIs
- MongoDB for persistent storage of accounts, profiles, and telemetry
- Redis for caching and background jobs
- SDNS/Unbound for DNS resolution and policy enforcement

**Frontend**
- React + TypeScript SPA (shadcn/ui & Radix UI component system)
- Tailwind CSS for utility-first styling

**Infrastructure**
- Docker & Docker Compose for local orchestration
- Nginx as the public ingress & TLS termination layer
- GitHub Actions for CI/CD automation

## Features

**Encrypted DNS**
- DNS over HTTPS (DoH)
- DNS over TLS (DoT)
- DNS over QUIC (DoQ)

**Content Filtering**
- Built-in blocklists (ads, malware, trackers)
- Custom allow/deny rules per profile

**User & Profile Management**
- Multi-profile accounts with individualized policies
- MFA, email verification, and secure password workflows

**Observability**
- Near real-time DNS query logging
- Exportable analytics for auditing

**Apple Device Integration**
- Managed `.mobileconfig` profiles
- QR-code assisted enrollment for mobile clients

## Getting Started

### Prerequisites
- Docker & Docker Compose
- Make (for the provided automation scripts)
- Node.js 18+ and npm (for the React application)
- Go 1.25+ (for backend services)
- mkcert (optional, for trusted local TLS certificates)

### Quick Start

```bash
make up          # Build and start every service stack
make down        # Stop and remove containers
```

Certificates for local HTTPS access live in `certs/`. You can either generate them with `mkcert` or import the bundled CA into your OS trust store.

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
| `bootstrap/`, `compose.*.yml` | Docker-compose orchestration and bootstrap assets

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

### API service (`api/`)
```bash
cd api
go mod tidy
make test
```
(See `api/Makefile` for additional targets like `make lint`, `make dev`, etc.)

### Proxy service (`proxy/`)
```bash
cd api
go mod tidy
make test
```
(See `proxy/Makefile` for additional targets like `make lint`, `make dev`, etc.)

### Integration tests (`tests/`)
```bash
python -m venv tests/venv
source tests/venv/bin/activate
pip install -r tests/requirements.txt
make test_ci     # spins up containers via testcontainers
```

## Testing

- **Web client**: `npm run lint && npm run tsc && npm run test` (unit) and `npm run test:e2e` (Playwright)
- **Go services**: `go test ./...` inside each Go module (`api/`, `blocklists/`, `proxy/`, etc.)
- **Integration**: `source tests/venv/bin/activate && make test_ci`
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

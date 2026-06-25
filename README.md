# Convocate

Convocate is an agentic software-engineering platform. A user describes an
application end-state in natural language; Convocate decomposes that
specification into an execution graph, runs AI agents on managed compute to
implement it, and manages the resulting code, reviews and deployments — all
from a single desktop-style web UI.

## Architecture

Convocate runs as four containers orchestrated via Docker Compose:

| Container  | Purpose                          | Runtime     |
|------------|----------------------------------|-------------|
| **UI**     | SpecifyJS SPA (Bun)              | distroless  |
| **API**    | Go REST server                   | distroless  |
| **Redis**  | In-memory cache and sessions     | distroless  |
| **PostgreSQL** | Searchable records and references | distroless |

The user-facing product is a **Unity/GNOME-style desktop** in the browser
with seven applets:

1. **Node Manager** — provision and manage compute hosts
2. **Agent Manager** — orchestrate AI agent containers on nodes
3. **Project Board** — decompose specs into execution DAGs of cards and containers
4. **Code IDE** — edit code, specifications and configurations
5. **Access Control** — users, groups, roles and security settings
6. **Repo Manager** — git repositories, pull requests and CI/CD
7. **Support Tool** — tickets and documentation

## Prerequisites

- Docker and Docker Compose
- Go 1.26+ (for local API development)
- Bun (for local UI development)

## Quick Start

```bash
docker compose up --build
```

## Documentation

- [SPECIFICATION.md](SPECIFICATION.md) — product specification and UI behavior
- [openapi.yaml](openapi.yaml) — API contract (authoritative)
- [CLAUDE.md](CLAUDE.md) — project instructions and coding standards
- [CONTRIBUTING.md](CONTRIBUTING.md) — contribution guidelines
- [SECURITY.md](SECURITY.md) — security policy

## License

[MIT](LICENSE.txt)

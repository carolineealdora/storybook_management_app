# Backend Code Base

**Version:** v1.0

> **Engineers: before writing a single line of code, read [`engineering-with-claude-playbook.md`](engineering-with-claude-playbook.md).** It defines the mandatory workflow for using Claude Code on this project — plan mode, TDD, quality gates, and session management. This is not optional reading.

---

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Language | Go 1.20 |
| HTTP Framework | [Echo v4](https://echo.labstack.com/) |
| Database | PostgreSQL (via GORM) + MongoDB |
| Cache | Redis |
| Auth | JWT |
| Logging | Logrus + Elastic APM |

---

## Architecture

This codebase follows a **3-layer architecture** — every feature is implemented as a stack of three distinct layers, each with a single responsibility.

```
HTTP Request
     │
     ▼
┌─────────────────────────────────┐
│  Handler Layer                  │  handler/controllers/
│  Parses request, validates      │
│  input, calls service,          │
│  formats HTTP response          │
└────────────────┬────────────────┘
                 │ calls
                 ▼
┌─────────────────────────────────┐
│  Service Layer                  │  internal/services/
│  Contains business logic.       │
│  Orchestrates repository calls. │
│  No HTTP or DB concerns here.   │
└────────────────┬────────────────┘
                 │ calls
                 ▼
┌─────────────────────────────────┐
│  Repository Layer               │  internal/repositories/
│  Talks to the database.         │
│  No business logic here —       │
│  only data access (queries).    │
└─────────────────────────────────┘
```

**Rule:** Business logic only lives in the service layer. Handlers are thin HTTP adapters. Repositories are thin DB adapters. Nothing crosses these boundaries.

---

## Directory Structure

```
.
├── main.go                   # Entry point — boots server or runs migrations
├── handler/
│   ├── route.go              # Route registration
│   └── controllers/          # Handler layer — one package per domain
│       ├── healthcheck/
│       └── payment/
├── internal/
│   ├── services/             # Service layer — business logic
│   │   ├── healthcheck/
│   │   └── payment/
│   ├── repositories/         # Repository layer — DB access
│   │   ├── healthcheck/
│   │   └── payment/
│   ├── helper/               # Shared internal utilities
│   └── constant/             # Shared constants
├── models/                   # Data structures, DTOs, SharedHolder
├── pkg/                      # Reusable infrastructure packages
│   ├── logger/               # Logger interface + Logrus implementation
│   ├── orm/                  # DB connection (GORM + Postgres)
│   ├── migrator/             # DB migration runner
│   ├── monitor/              # APM / observability
│   └── shared/               # Shared bootstrapping utilities
├── config/                   # Env var loading and config struct
├── migrations/               # DB migration runner wiring
├── test/mock/                # Mocks for unit testing
├── docs/                     # Architecture docs, API contracts, PRD templates
├── .claude/rules/            # Claude Code behavioral rules
├── CLAUDE.md                 # Claude Code project instructions
├── PLAN.md                   # Active implementation plan
├── tech-debt.md              # Deferred work registry
└── engineering-with-claude-playbook.md   # ← READ THIS FIRST
```

---

## Running Locally

```bash
cp .env.example .env          # then fill in JWT_SECRET and the SEED_* passwords
make db-up                    # Postgres 16 in Docker on :5433 (+ storybook_test DB)
make migrate                  # go run main.go migrate up
make seed                     # mocked admin + parent (for storybook_library_app) and the default plan
make run                      # back-office API on :9999
```

API reference: [docs/API-contracts.md](docs/API-contracts.md). Architecture: [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

---

## Build for Production

```bash
go get -d -v ./...
go build -o /go/bin/app
```

---

## Run Tests

```bash
make test               # unit tests (services, controllers, helpers) with -race and coverage
make test-integration   # repository tests against Postgres (needs make db-up)
```

---

## Quality Gate

Before committing anything, run:

```bash
make done
```

This runs `lint` (go vet), `test`, `build` and `secrets` (detect-secrets; skipped with a notice if not installed) in one command. **Never commit if it fails.**

---

## Environment Variables

Copy `.env.example` to `.env` and fill in real values. Never commit `.env`.

| Name | Default | Description |
|------|---------|-------------|
| `ENV` | `development` | Runtime environment |
| `PORT` | `9999` | HTTP port |
| `DB_HOST` / `DB_PORT` / `DB_NAME` / `DB_USERNAME` / `DB_PASSWORD` | `localhost` / `5432` / `storybook` / `postgres` / `postgres` | PostgreSQL connection (`make db-up` uses port `5433`) |
| `JWT_SECRET` | — (required) | ≥ 32 chars; the server refuses to start without it. Must differ from storybook_library_app's |
| `ACCESS_TOKEN_TTL` / `REFRESH_TOKEN_TTL` | `30m` / `168h` | Token lifetimes |
| `CORS_ALLOW_ORIGINS` | `*` | Comma-separated frontend origins |
| `SEED_PARENT_EMAIL` / `SEED_PARENT_PASSWORD` / `SEED_ADMIN_EMAIL` / `SEED_ADMIN_PASSWORD` | — | Accounts created by `make seed` |
| `SEED_PLAN_PRICE` / `SEED_PLAN_BENEFITS` | `49000` / `Read all published books.` | Initial subscription plan |

---

## Port

| Port | Description |
|------|-------------|
| `9999` | API service |

---

## docs/ — Templates and References (Customize as Needed)

The `docs/` directory contains architecture documentation, API contract specs, data model references, and PRD templates:

```
docs/
├── ARCHITECTURE.md       # System overview and key decisions
├── API-contracts.md      # Endpoint specs and request/response shapes
├── DATA-MODELS.md        # Entity definitions and relationships
├── DECISIONS.md          # Architectural decision log
└── PRD-[feature].md      # Per-feature product brief template
```

> **These documents are starting points, not mandates.** Adapt the structure, level of detail, and format to fit your team and feature. The goal is for the team and Claude to have shared context — not to maintain perfect documentation for its own sake. Skip sections that don't apply. Expand sections that matter for your feature.

The `.claude/rules/` files follow the same principle — they are defaults that can and should be adjusted as the project evolves.

---

## Claude Code Setup (Required for AI-assisted Development)

This project uses Claude Code as a core development tool. Install the following two plugins **once** before your first session — they apply to every session after that.

### caveman — output compression

Cuts ~65–75% of Claude's output tokens by compressing prose while keeping all code, errors, and technical detail intact. Reduces noise and cost across every session.

```bash
curl -fsSL https://raw.githubusercontent.com/JuliusBrussee/caveman/main/install.sh | bash
```

Mid-session controls (type inside Claude Code chat):

| Command | Effect |
|---------|--------|
| `/caveman lite` | Light compression |
| `/caveman` | Default compression |
| `/caveman ultra` | Maximum compression |
| `normal mode` | Disable compression |

### ponytail — minimal-code discipline

Enforces "laziest senior dev" coding: stdlib before third-party, no unasked-for abstractions, deletion over addition. Keeps Claude from over-engineering.

Inside a Claude Code session, run these commands in order:

```
/plugin marketplace add DietrichGebert/ponytail
/plugin install ponytail@ponytail
```

Then go to `/hooks`, trust the two lifecycle hooks, and start a new thread.

Mid-session controls:

| Command | Effect |
|---------|--------|
| `/ponytail-review` | Scans current diff for over-engineering, returns a delete-list |
| `/ponytail ultra` | Questions whether the task needs to exist at all |

> Both tools are Claude Code plugins — they operate at the AI layer, not the code layer. They are not Go libraries and do not affect the compiled binary.

---

## Engineering Workflow

This project uses Claude Code as a core development tool. The full workflow — including plan mode, TDD, quality gates, multi-session management, and context hygiene — is documented in:

**[`engineering-with-claude-playbook.md`](engineering-with-claude-playbook.md)**

Every engineer working on this codebase is expected to read it before starting any implementation work.

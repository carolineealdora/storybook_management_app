# Architecture Overview — Storybook Back-Office Backend
**Last updated:** 2026-11-16
**This is the map, not the manual.** Product rules: [PRD_storybook-lib-app.md](PRD_storybook-lib-app.md). Endpoints: [API-contracts.md](API-contracts.md). Schema: [DATA-MODELS.md](DATA-MODELS.md). Decisions: [DECISIONS.md](DECISIONS.md).

## System purpose
This Go API serves only the **Book Management** frontend (`fe_storybook_management_app`). An admin manages the book catalogue and the monthly subscription plan.

The parent and child experience (showcase, reading, buying, subscribing, PIN, auto-renew) is served by the separate **storybook_library_app** backend for `fe_storybook_library_app` ([ADR-011](DECISIONS.md#adr-011)).

Both backends share one PostgreSQL database. This repo owns the schema; the library app runs no migrations ([ADR-010](DECISIONS.md#adr-010)).

Login is **mocked**: there is no sign-up, and seeded accounts come from `make seed`.

## Tech stack
| Layer | Technology |
|---|---|
| API | Go 1.22, Echo v4 |
| Database | PostgreSQL 16 via GORM (wrapped by `pkg/orm`). Stores app data **and** EPUB/cover bytes |
| Auth | HS256 JWT access token (30 min) + opaque refresh token (7 days, SHA-256 stored). Admin accounts only |
| Background work | None. The renewal job runs in storybook_library_app |
| Observability | Echo access log (no bodies), Logrus JSON logs, Elastic APM middleware |

No Redis, no Mongo, no object storage, no queue (PLAN D3).

## Request flow
```
Client ──HTTPS──▶ Echo middleware (recover, request id, CORS, APM, access log)
                   └▶ RequireAccount(admin)   401 / 403 here
                       └▶ Controller   handler/controllers/<domain>   bind + validate, map errors
                           └▶ Service  internal/services/<domain>     business rules
                               └▶ Repository internal/repositories/<domain>  SQL, transactions
                                   └▶ PostgreSQL
```
- Controllers are thin. Services own every PRD rule. Repositories own SQL and transactions.
- Errors are created with `helpers.Error.Wrap(CODE, internal detail)`. `helpers.Response` turns the code into the HTTP status and user message and logs the detail.

## Package map
| Domain | Controller | Service | Repository |
|---|---|---|---|
| Auth (admin login, refresh, logout) | `auth` | `auth` | `account` |
| Back-office books and covers | `adminbook` | `adminbook` | `book` |
| Subscription plan and price history | `subscription` | `subscription` | `subscription` |

Shared: `internal/helper` (response, binding, JWT and middleware), `internal/constant/response` (error code → HTTP map), `models` (DB rows), `models/dto` (API shapes), `pkg/orm` (DB wrapper, `WithTx`, error sentinels).

## Critical paths
**Change the plan price** (`PUT /admin/subscription-plan`): one transaction updates `subscription_plans` and inserts a `subscription_plan_price_history` row (only when the price changes). New subscribers pay the new price at once. The library app charges it to active subscribers at their next renewal.

**Edit a book** (`PUT /admin/books/:id`): new prices apply to new orders only, because orders snapshot the price at checkout. Unpublishing or deleting hides the book from the showcase immediately; owners keep it in My Book.

**Refresh** (`POST /auth/refresh`): the `refresh_tokens` table is shared, so a token whose credential is not `admin` gets `401 UNAUTHORIZED`.

## Repository layout
```
main.go                      run | migrate up|down|create | seed
handler/route.go             all routes + middleware (route_test.go sweeps auth on every route)
handler/controllers/<domain>/
internal/services/<domain>/
internal/repositories/<domain>/   *_integration_test.go need Postgres (make test-integration)
internal/helper/  internal/constant/
models/  models/dto/
migrations/*.sql  migrations/seed.go     shared schema; seed also creates the parent used by the library app
pkg/orm  pkg/logger  pkg/migrator  pkg/monitor
test/mock/  test/apitest/
```

## Configuration
Environment variables only; see [.env.example](../.env.example). `JWT_SECRET` (≥ 32 chars) is required and the server refuses to start without it. Use a different `JWT_SECRET` from storybook_library_app.

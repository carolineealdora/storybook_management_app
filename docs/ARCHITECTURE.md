# Architecture Overview — Storybook Library Backend
**Last updated:** 2026-11-16
**This is the map, not the manual.** Product rules: [PRD_storybook-lib-app.md](PRD_storybook-lib-app.md). Endpoints: [API-contracts.md](API-contracts.md). Schema: [DATA-MODELS.md](DATA-MODELS.md). Decisions: [DECISIONS.md](DECISIONS.md).

## System purpose
This Go API is called by the **Book Management** frontend (`fe_storybook_management_app`). It also still implements the parent endpoints below, but the parent frontend (`fe_storybook_library_app`) calls storybook_library_app instead (TD-12):
- **Digital Storybook Library app**: parents and children (1–6 y) browse, buy, subscribe, and read EPUB picture books. Parent and Child views share one account; a PIN gates Child → Parent.
- **Book Management app**: an admin manages the book catalogue and the monthly subscription plan.

Login and payment are **mocked** (seeded accounts; pay endpoints take `success`/`failed`).

**Shared database:** the separate `storybook_library_app` backend reads and writes the same PostgreSQL database. This repo owns the schema; that app runs no migrations ([ADR-010](DECISIONS.md#adr-010)). Both run the renewal job, which stays safe because of the conditional update and the `renewal:<id>` idempotency key.

## Tech stack
| Layer | Technology |
|---|---|
| API | Go 1.22, Echo v4 |
| Database | PostgreSQL 16 via GORM (wrapped by `pkg/orm`). Stores app data **and** EPUB/cover bytes |
| Auth | HS256 JWT access token (30 min) + opaque refresh token (7 days, SHA-256 stored) |
| Background work | In-process ticker for subscription auto-renew (hourly) |
| Observability | Echo access log (no bodies), Logrus JSON logs, Elastic APM middleware |

No Redis, no Mongo, no object storage, no queue (PLAN D3).

## Request flow
```
Client ──HTTPS──▶ Echo middleware (recover, request id, CORS, APM, access log)
                   └▶ RequireAccount(parent|admin)   401 / 403 here
                       └▶ Controller   handler/controllers/<domain>   bind + validate, map errors
                           └▶ Service  internal/services/<domain>     business rules
                               └▶ Repository internal/repositories/<domain>  SQL, transactions
                                   └▶ PostgreSQL
```
- Controllers are thin. Services own every PRD rule. Repositories own SQL, including the transactions and conditional updates that keep money paths race-safe.
- Errors are created with `helpers.Error.Wrap(CODE, internal detail)`. `helpers.Response` turns the code into the HTTP status and user message and logs the detail.

## Package map
| Domain | Controller | Service | Repository |
|---|---|---|---|
| Auth (login/refresh/logout) | `auth` | `auth` | `account` |
| Profile & PIN | `account` | `account` | `account`, `subscription` |
| Library (showcase, detail, cover, EPUB, My Book) | `book` | `book` | `book`, `order`, `subscription` |
| Back-office books | `adminbook` | `adminbook` | `book` |
| Book orders | `order` | `order` | `book`, `order`, `subscription` |
| Subscription (plan, orders, cancel, history, renew job) | `subscription` | `subscription` | `subscription` |

Shared: `internal/helper` (response, binding, JWT and middleware), `internal/constant/response` (error code → HTTP map), `models` (DB rows), `models/dto` (API shapes), `pkg/orm` (DB wrapper, `WithTx`, error sentinels).

## Critical paths
**Read a book** (`GET /books/:id/content`): load book with EPUB → access check (free → owned → subscribed → locked) → stream bytes or `403 BOOK_LOCKED`. Locked content never leaves the server.

**Buy a book**: `POST /book-orders` snapshots the price → `POST /book-orders/:id/pay` re-checks availability, ownership, and subscription → in one transaction, `pending → paid` (conditional) and insert `purchases`. A double tap loses the conditional update and gets `409 DUPLICATE_REQUEST`.

**Subscribe**: `POST /subscription-orders` → pay → one transaction inserts the `active` period (unique partial index = one active per user) and marks the order paid.

**Auto-renew** (`subscription.StartRenewalJob`, every `RENEWAL_INTERVAL`, default 1h): for each active period with `expires_at <= now`, one transaction expires the old period, inserts the next period (starting at the old `expires_at`, at the current plan price), and inserts a paid `renewal` order.

## Repository layout
```
main.go                      run | migrate up|down|create | seed
handler/route.go             all routes + middleware (route_test.go sweeps auth on every route)
handler/controllers/<domain>/
internal/services/<domain>/
internal/repositories/<domain>/   *_integration_test.go need Postgres (make test-integration)
internal/helper/  internal/constant/
models/  models/dto/
migrations/*.sql  migrations/seed.go
pkg/orm  pkg/logger  pkg/migrator  pkg/monitor
test/mock/  test/apitest/
```

## Configuration
Environment variables only; see [.env.example](../.env.example). `JWT_SECRET` (≥ 32 chars) is required and the server refuses to start without it.

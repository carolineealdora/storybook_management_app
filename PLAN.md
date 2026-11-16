# Plan: Storybook Library — Backend Service
**Source:** [docs/PRD_storybook-lib-app.md](docs/PRD_storybook-lib-app.md) (US-01 … US-17, edge cases E1–E22, §12 system design)
**Branch:** `feat/STORY-1-backend-service` (no ticket ID was provided, so STORY-1 is a placeholder; rename with `git branch -m`)
**Status:** DONE. All phases implemented. Decisions D1, D3, D4, D5, D7 confirmed by the engineer on 2026-11-16; Risk 2 approved (docs replaced).
**Last updated:** 2026-11-16

---

## Phases
Each phase is two commits: `test(scope): …` (failing tests, failure output shown), then `feat(scope): …` (implementation, `make done` green).

- [x] **Phase 0: Scaffold fixes** (`chore`)
- [x] **Phase 1: Schema + seed** (`db`)
- [x] **Phase 2: Auth**: parent and admin login, refresh, logout, JWT + role middleware (`auth`)
- [x] **Phase 3: Profile & PIN**: `/me`, PIN set, verify, and forgot, with lockout (`account`)
- [x] **Phase 4: Back office**: book CRUD with file upload, subscription plan config (`admin`)
- [x] **Phase 5: Library**: showcase with access status, detail, cover, EPUB stream, My Book (`book`)
- [x] **Phase 6: Book orders**: Buy Now, mock pay, Order History (`order`)
- [x] **Phase 7: Subscriptions**: plan, subscribe, mock pay, cancel, history, auto-renew job (`subscription`)
- [x] **Phase 8: Docs + done report**: real API contract and data model docs, coverage ≥ 80%

---

## Phase 0: Scaffold fixes (bugs found while reading the scaffold)
| Problem found | Fix |
|---|---|
| `Makefile` is a **directory**, not a file. No `make` targets exist, so `make done` cannot run. | Replace it with a real `Makefile` with `run`, `migrate`, `seed`, `test` (`-race -cover`), `test-integration`, `lint`, `build`, and `done`. |
| The migrator's run order comes from Go **map iteration**, which is random, so migrations can apply out of order. | Use the ordered `migrations` slice. |
| `migrations/20250806084945_create_payment_table.sql` is listed but missing, so `migrate up` fails. | Delete it, together with the loan/partner `payment` example (controller, service, repository, mocks, models, and the `BILL_NOT_FOUND`/`PARTNER_NOT_FOUND` constants). |
| The global middleware requires an `x-partner-id` header on every route. | Remove it (it's from another domain). Keep `x-request-id` optional and generate one when missing. |
| The Redis "request-id lock" middleware ignores errors and blocks a retry when the same request id is reused. | Remove it. Idempotency is handled by a DB unique key (see D5). |
| `BodyDump` writes full request and response bodies to **Mongo**, which would log passwords, PINs, tokens, and EPUB bytes. This breaks the "never log sensitive fields" rule. | Remove it, along with the Mongo client and config. The echo access log stays. |
| `main.go`: `InitRoute` blocks on `Start`, and `run()` only prints a log line. | `InitRoute` registers routes only. `run()` starts the scheduler and the server, with graceful shutdown. |
| The response envelope is `{responseCode, responseDesc, responseData}`, but PRD §12.4 specifies `{error:{code,message}}`. | See D1. |
| `go.mod` says `go 1.20`; the project targets Go 1.22. | Bump to 1.22. |

## Phase 1: Schema (single initial migration file covering every PRD §12.2 table, UUID PKs, `INTEGER` IDR, `VARCHAR`+`CHECK` enums)
`users`, `admins`, `credentials`, `refresh_tokens`, `books` (`epub_content BYTEA`, `deleted_at`), `book_files` (cover, `UNIQUE(book_id)`), `subscription_plans` (single row), `subscriptions`, `book_orders` (`UNIQUE(idempotency_key)`), `subscription_orders` (`UNIQUE(idempotency_key)`), `purchases` (`UNIQUE(user_id, book_id)`).
Columns added beyond the PRD:
- `users.pin_failed_attempts INT` and `users.pin_locked_until TIMESTAMPTZ` for E12. These live in the DB, not Redis (D3).
- A partial unique index `subscriptions(user_id) WHERE status='active'`, so the DB itself guarantees at most one active subscription per user.

Seed: `go run main.go seed` reads `SEED_PARENT_EMAIL/PASSWORD` and `SEED_ADMIN_EMAIL/PASSWORD` from env and creates one parent, one admin, and the default plan. No credentials are committed.

## Phase 2–7: Endpoints (exactly PRD §12.3, under `/api/v1`)
**Library (parent JWT):** `POST /auth/login|refresh|logout` · `GET/PATCH /me` · `PUT /me/pin` · `POST /me/pin/verify|forgot` · `GET /books`, `/books/:id`, `/books/:id/cover`, `/books/:id/content` · `GET /me/books` · `POST/GET /book-orders`, `POST /book-orders/:id/pay` · `GET /subscription/plan` · `POST /subscription-orders`, `POST /subscription-orders/:id/pay` · `POST /subscriptions/current/cancel` · `GET /subscriptions`

**Back office (admin JWT):** `POST /admin/auth/login|logout` · `GET/POST /admin/books`, `GET/PUT/DELETE /admin/books/:id` · `GET/PUT /admin/subscription-plan`

### Core rules (service layer)
- **Access check** (PRD §12.1 order): book exists, is published, and is not deleted, else `404 BOOK_NOT_FOUND`; then `price == 0` → `free`; purchased → `owned`; active subscription → `subscribed`; otherwise `locked`. **Exception:** a purchased book that was later soft-deleted or unpublished stays readable through `/books/:id/content` and stays in My Book (E5).
- **Book order create:** rejected when the book is free (E10 `BOOK_NOT_PURCHASABLE`), already owned (E1 `BOOK_ALREADY_OWNED`), or the user is subscribed (E2 `SUBSCRIPTION_ACTIVE`). Price is snapshotted onto the order.
- **Book order pay:** re-checks the book is still available (E6 `BOOK_UNAVAILABLE`) and re-runs the E1 and E2 checks. It charges **the order's snapshot price, not the current price** (E7). It works only when status is `pending`; otherwise `409 DUPLICATE_REQUEST` (E8). On success, the `paid` order and the `purchases` row are written in **one transaction**. On failure, the order is `failed` and the response is `402 PAYMENT_FAILED`.
- **Subscription pay success:** creates an `active` subscription with `expires_at = started_at + 1 month` and `price_paid` from the order snapshot. On failure, no subscription is created.
- **Cancel:** status becomes `cancelled` and `cancelled_at = now`, and access stops immediately. If nothing is active: `409 SUBSCRIPTION_NOT_ACTIVE` (E14).
- **Auto-renew job:** an in-process ticker (hourly) finds `active` subscriptions with `expires_at <= now`. In one transaction per subscription: the old row is set to `expired` (conditional `UPDATE … WHERE status='active'`, so two runs cannot both renew it), and a new `active` row is created with `renewed_from_id` and the **current** plan price, plus a `renewal` subscription order that is already `paid`.
- **PIN:** must be 4 digits and is stored as a bcrypt hash. Changing an existing PIN requires the account password. Errors: verify before any PIN is set → `409 PIN_NOT_SET` (E11); wrong PIN → `401 INVALID_PIN`; the 5th wrong attempt locks verification for 1 minute → `429 TOO_MANY_ATTEMPTS` (E12). Forgot PIN with a wrong password → `401 INVALID_PASSWORD` (E13).
- **Admin book validation:** cover must be `.jpg/.jpeg/.png` and the book file `.epub`, checked by extension **and** magic bytes, else `400 INVALID_FILE_FORMAT` (E15). Age range: `range` requires whole numbers with `0 ≤ min < max`; `plus` requires `min` only; violations → `400 INVALID_AGE_RANGE` (E16). Price must be a whole number ≥ 0. A computed `age_label` (`"1–3"` / `"3+"`) is returned.
- **List queries never select `epub_content`** (PRD §15 risk).

### Layout (follows the scaffold's handler → service → repository split)
```
handler/controllers/{auth,account,book,adminbook,order,subscription}/
internal/services/{auth,account,book,order,subscription}/      # + scheduler in subscription
internal/repositories/{credential,user,book,order,subscription}/
internal/helper/{response.go,jwt.go,auth_middleware.go}          # modified / added
models/*.go, models/dto/*.go
migrations/*.sql
```
One file per domain layer (`controller.go`, `service.go`, `repository.go` plus `_test.go`), **not** the scaffold's one-file-per-method. That alone avoids about 60 near-empty files.

**Files NOT touched:** `pkg/logger`, `pkg/monitor`, `pkg/orm` (used as is), `.claude/rules/*`, and `engineering-with-claude-playbook.md`.

---

## Decisions (please confirm or annotate)
| # | Decision | Default I'll use | Why |
|---|---|---|---|
| D1 | Response envelope | Success: `{"data": …}` (lists add `meta.total`). Error: `{"error":{"code","message"}}` with the PRD codes. I'll rewrite `helper/response.go` and the code→HTTP map, keeping the `Response` interface. | The PRD is the contract for the Lovable frontend. The scaffold envelope is from a different service. |
| D2 | JWT signing | **HS256** with `JWT_SECRET` from env; `golang-jwt/jwt/v5`. Access token 30 min, refresh token 7 days, opaque, SHA-256 hash stored in `refresh_tokens`. | A single service issues and verifies tokens. RS256 only pays off when other services verify them. |
| D3 | Infra dependencies | **PostgreSQL only.** Drop Redis and Mongo. | Nothing in the PRD needs them. PIN lockout fits in two columns, and idempotency is a unique index. One fewer system to run (matches PRD §11). |
| D4 | Mock payment input | `POST …/pay` body `{"result": "success" \| "failed"}` | The PRD says the mock has exactly two outcomes, and the client chooses which one. |
| D5 | Idempotency (E8) | The client sends `idempotency_key` when creating an order. A repeated key → `409 DUPLICATE_REQUEST`. Paying a non-`pending` order → `409 DUPLICATE_REQUEST`. **Retry after a failed payment creates a new order** with a new key, so Order History shows both the Failed and the Paid attempt. | US-12 lists Paid and Failed rows, which implies one row per attempt. |
| D6 | "Subscribed" definition | `status='active'`, without comparing against `expires_at` | Renewal always succeeds (mock), so checking the date would only lock users out in the gap before the job runs. |
| D7 | View mode and "Settings only from Parent view" | **Client-side only.** The API does not know the current view mode; it enforces the PIN check and lockout. | Parent and child share one account and one token (PRD assumption), so the server cannot tell who is holding the device. |
| D8 | Pagination | None on `/books` (PRD: "all books in one showcase"). The admin list also returns everything for the MVP. Logged in `tech-debt.md`. | The catalogue is small for the MVP. |
| D9 | Upload limits | Cover ≤ 5 MB, EPUB ≤ 50 MB; enforced with echo `BodyLimit` on admin routes. `PUT /admin/books/:id` keeps the existing files when none are sent. | Files are stored in Postgres, so the request size needs a cap. |
| D10 | Tests | **Service:** testify mocks of repository interfaces, covering every AC and E1–E22. **Handler:** `httptest` + mocked service, asserting the status and error code for each mapping, plus 401/403 middleware tests. **Repository:** `//go:build integration` against Postgres in Docker (`make test-integration`), not run before commits. Hand-written mocks in `test/mock/`, as in the scaffold (`mockery` isn't installed). | Follows `.claude/rules/testing.md`. |

## Risks / open questions
1. **Docker isn't running** on this machine. Repository integration tests and manual smoke tests need it (`docker run postgres:16`). Unit tests don't.
2. **`docs/ARCHITECTURE.md`, `API-contracts.md`, `DATA-MODELS.md`, and `DECISIONS.md` describe "Viva Sports Club"** (a template). CLAUDE.md loads them as context, which is misleading. Phase 8 replaces API-contracts and DATA-MODELS with the storybook versions and trims ARCHITECTURE. OK to overwrite?
3. **`.claude/rules/api-patterns.md` conflicts with `security.md`** (cursor vs offset pagination, different error shape). I'm following the PRD and `security.md`. The rules files aren't touched; flagging only.
4. A child holding the parent's token can call parent endpoints directly (D7). This is accepted per the PRD; real protection needs child profiles (out of scope).
5. Single-instance scheduler: the conditional update prevents double renewal, but with several instances every one of them would scan. Logged as tech debt.

## Tech-debt entries this plan will add
EPUB/cover stored in the DB · no pagination · HS256 shared secret · in-process scheduler · no rate limit on `/auth/login` (only the PIN has a lockout) · mocked auth and payment

## Deviations from this plan (made during implementation)
- Phase 1: one initial migration file instead of one per table. It is the first schema and is applied as a unit.
- Phase 0/1: `pkg/orm` gained `WithTx`, `NewPostgresDSN`, `ErrDuplicate`/`ErrStateChanged` and `IsUniqueViolation` (planned as untouched).
- Dev/test Postgres runs on **:5433**, because a local Postgres already owned :5432.
- `/auth/refresh` serves both apps; the admin app has no separate refresh endpoint.
- `GET /books/:id/cover` accepts parent **and** admin tokens, so the admin app can show covers.
- Additional removals: unused resty HTTP client and the `Restful` mock (they failed `go vet`).

---

# Plan: CR-001 — Account badge & subscription price change
**Source:** [docs/CR-001_Badge_and_Subscription_Pricing.md](docs/CR-001_Badge_and_Subscription_Pricing.md)
**Branch:** `feat/STORY-2-cr001-badge-plan-pricing` (no ticket; STORY-2 is a placeholder), cut from `feat/STORY-1-backend-service`
**Status:** DONE. Approved 2026-11-16 (C1, C4, C7 confirmed; other conflicts resolved as recommended)

## Already in place (no change)
`subscription_plans`, `subscriptions.plan_id/price_paid`; checkout snapshots the current plan price; renewal charges the latest price (P1–P7, T4–T6, T9, B4, B5).

## Decisions
| # | Decision |
|---|---|
| C1 | Keep the mocked hourly auto-renew (PRD US-10). No manual `/payments/subscription/renew`, so no `NO_ACTIVE_SUBSCRIPTION` (T10 dropped). |
| C2 | Keep existing paths (`/subscription/plan`, `/subscription-orders`, `/:id/pay`). |
| C3 | Renewal keeps creating a new period row (PRD US-13); no `renewed_at`. |
| C4 | `plan_status` = `subscribed` when a `status = 'active'` period exists, same rule as the access check (ADR-006). |
| C5 | Keep calendar-month duration; no `duration_days`. |
| C6 | `updated_by` / `changed_by` reference `admins.id`. |
| C7 | `expected_price` required on `POST /subscription-orders` (409 `PRICE_CHANGED` on mismatch). Pay returns 409 `PRICE_CHANGED` when the order price ≠ current plan price; the order stays pending, client creates a new one. |
| C8 | Price ≤ 0 → 400 `INVALID_PRICE` (checked in the service). |
| C9 | `/me` adds `plan_status`; `is_subscriber` kept (deprecated). |
| C10 | Same price → no history row (E-CR6); same price and benefits → no write at all. First-ever plan creation writes no history row. |

## Phases
- [x] **Phase A (db):** migration `20261117000001_cr001_plan_price_history.sql`; `SavePlan` writes a history row in the same transaction when the price changes; `ListPriceHistory`.
- [x] **Phase B (subscription):** `INVALID_PRICE`, `PRICE_CHANGED`; `UpdatePlan(adminID, …)`; `GetAdminPlan` with price history; `expected_price` check on create; plan-price check on pay.
- [x] **Phase C (account):** `/me` → `plan_status`, `subscription.next_renewal_price`, `subscription.price_change_pending`.
- [x] **Phase D (docs):** API-contracts, DATA-MODELS, DECISIONS (ADR-009).

## Not touched
Auth, book orders/payment (P7), adminbook, book, renewal scheduler, the existing migration file, seed.

## Risks
- Price check on pay and `Activate` are not one transaction: an admin change in that millisecond window is charged at the order price. Acceptable for a mock payment.
- `expected_price` is a breaking change for existing `POST /subscription-orders` clients.

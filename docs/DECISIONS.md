# Architectural Decisions — Storybook Library Backend
**Last updated:** 2026-11-16

ADR format: what was decided, why, and what would change it. Never delete an ADR; supersede it.

| ADR | Title | Status |
|---|---|---|
| [ADR-001](#adr-001) | PostgreSQL only; EPUB and covers stored in the DB | Accepted |
| [ADR-002](#adr-002) | PRD error envelope instead of the scaffold envelope | Accepted |
| [ADR-003](#adr-003) | HS256 access JWT + DB-stored refresh tokens | Accepted |
| [ADR-004](#adr-004) | Race safety through DB constraints and conditional updates | Accepted |
| [ADR-005](#adr-005) | One order per payment attempt; client-chosen mock result | Accepted |
| [ADR-006](#adr-006) | "Subscribed" = status active; in-process renewal job | Accepted |
| [ADR-007](#adr-007) | View mode is client-side; server enforces the PIN only | Accepted |
| [ADR-008](#adr-008) | No pagination for the MVP | Accepted |
| [ADR-009](#adr-009) | CR-001 mapped onto the existing subscription model | Accepted |

---

<a name="adr-001"></a>
## ADR-001 — PostgreSQL only; EPUB and covers stored in the DB
**Decision:** One Postgres database holds app data, EPUBs (`books.epub_content`), and covers (`book_files`). The scaffold's Redis and Mongo were removed.
**Why:** PRD §11 chose DB file storage for MVP simplicity. No PRD feature needs a cache or document store, and the PIN lockout fits in two columns.
**Consequences:** List queries must never select `epub_content` (enforced by column lists). Requests are capped at 60 MB.
**Revisit when:** the catalogue size slows backups or downloads → move files to object storage with signed URLs.

<a name="adr-002"></a>
## ADR-002 — PRD error envelope
**Decision:** Success `{"data":…,"meta":…}`; error `{"error":{"code","message"}}` with PRD codes. The scaffold's `responseCode/responseDesc` envelope was replaced, keeping its `Response`/`Error.Wrap` pattern.
**Why:** the PRD is the contract the Lovable frontends are built against.

<a name="adr-003"></a>
## ADR-003 — HS256 access JWT + DB-stored refresh tokens
**Decision:** 30-minute HS256 access token (`JWT_SECRET`) and a 7-day opaque refresh token stored as a SHA-256 hash in `refresh_tokens`. Logout revokes it. No refresh-token rotation.
**Why:** a single service issues and verifies tokens, so RS256 adds key management with no benefit. DB storage makes logout real without Redis.
**Revisit when:** another service must verify tokens (→ RS256/JWKS), or stolen-refresh-token risk matters (→ rotation with reuse detection).

<a name="adr-004"></a>
## ADR-004 — Race safety through DB constraints and conditional updates
**Decision:** Invariants live in Postgres: one active subscription (partial unique index), one purchase per user and book, per-user idempotency keys. State changes use `UPDATE … WHERE status = '<expected>' RETURNING id`. Paying an order and recording the purchase or subscription happen in one transaction.
**Why:** double taps and concurrent requests are explicit PRD edge cases (E8). Checks done only in application code can race.

<a name="adr-005"></a>
## ADR-005 — One order per payment attempt; client-chosen mock result
**Decision:** `POST …/pay` takes `{"result":"success"|"failed"}`. A failed order stays `failed`; a retry creates a new order with a new idempotency key. Paying a non-pending order → `409 DUPLICATE_REQUEST`.
**Why:** PRD allows exactly two mock outcomes, and Order History lists both Failed and Paid rows.
**Revisit when:** a real gateway arrives → replace the result field with gateway initiation plus a signed webhook. Keep the order/transaction structure.

<a name="adr-006"></a>
## ADR-006 — "Subscribed" = status active; in-process renewal job
**Decision:** Access checks treat `status = 'active'` as subscribed, without comparing `expires_at`. A goroutine ticker (`RENEWAL_INTERVAL`, default 1h) renews ended periods back-to-back at the current plan price. Month math clamps (Jan 31 → Feb 28).
**Why:** mock renewal always succeeds, so a date check would only lock users out until the next tick. Conditional updates and the `renewal:<id>` idempotency key make concurrent runs safe.
**Revisit when:** real payments can fail on renewal (→ grace period + `past_due` status), or many instances run (→ one scheduled worker).

<a name="adr-007"></a>
## ADR-007 — View mode is client-side; server enforces the PIN only
**Decision:** The server does not know whether the app is in Parent or Child view. It verifies the PIN (with a 5-attempt / 1-minute lockout) and the Forgot-PIN password. Hiding prices, payment, and Settings in Child view is the client's job.
**Why:** PRD: one account, one token, no child profiles.
**Consequence:** a child holding the parent's token could call parent endpoints directly. Accepted for the MVP. The fix is a short-lived "parent mode" token issued by PIN verify and required by payment endpoints, or child profiles.

<a name="adr-008"></a>
## ADR-008 — No pagination for the MVP
**Decision:** List endpoints return every row with `meta.total`.
**Why:** PRD: "Home shows all published books in one showcase", and the catalogue is small.
**Revisit when:** any list grows past a few hundred rows (tracked as tech debt).

<a name="adr-009"></a>
## ADR-009 — CR-001 mapped onto the existing subscription model
**Decision:** CR-001 is implemented on the existing endpoints and tables instead of its proposed ones:
- Keep `/subscription/plan`, `/subscription-orders` and `/:id/pay`. No `/payments/subscription*` paths.
- Keep the mocked hourly auto-renew (PRD US-10). No manual renew endpoint, so no `NO_ACTIVE_SUBSCRIPTION`.
- Each renewal is still a new period row (PRD US-13), so there is no `renewed_at`. The period keeps a calendar month, so there is no `duration_days`.
- `plan_status` uses the same `status = 'active'` rule as book access (ADR-006), not `expires_at > now`.
- `updated_by` / `changed_by` reference `admins`, since admins are not in `users`.
- `expected_price` is required on order creation. Pay compares the order's snapshot price with the current plan price. Either mismatch returns `409 PRICE_CHANGED`.
**Why:** most of CR-001-2 (price locked per period, latest price at renewal) already held. Renaming endpoints or changing the renewal model would break the PRD contract for no behaviour gain. A date-based badge would disagree with book access during the gap before the renewal job runs.
**Revisit when:** renewals become user-triggered or can fail (then `expires_at` must count, and `past_due` appears), or a second plan is sold.

<a name="adr-010"></a>
## ADR-010 — This repo owns the schema shared with storybook_library_app
**Decision:** Both backends use one PostgreSQL database. Every migration lives in this repo's `migrations/`. The library app deleted its migrations and migrator; its integration test applies this repo's `migrations/*.sql` in filename order (`SCHEMA_DIR`, default `../storybook_management_app/migrations`). The library app's schema additions were merged in `20261117000002_merge_library_schema.sql`: the Forgot-PIN lockout columns, a unique `lower(email)` index, and three foreign-key indexes. Where the two schemas differed otherwise, this repo's definitions were kept (extra `created_at`/`updated_at`, wider text, status defaults), since the library app's writes already fit them. Seed accounts come only from `make seed` (`SEED_*` env vars). Frontend routing: `fe_storybook_management_app` → this backend, `fe_storybook_library_app` → storybook_library_app.
**Why:** two migration sets with different ids cannot share one `migrations` table. A second `migrate up` fails on `CREATE TABLE users`, and copying files between repos drifts again. Both Go modules are named `codebase`, so a shared Go package is not an option without renaming one.
**Revisit when:** a third service needs the schema, or CI comes back (then use a shared schema module or submodule instead of the sibling path).

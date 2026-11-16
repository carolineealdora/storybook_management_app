# Data Models — Storybook Library
**Last updated:** 2026-11-16
**Schema:** [migrations/20261116000001_create_storybook_schema.sql](../migrations/20261116000001_create_storybook_schema.sql) [20261117000001_cr001_plan_price_history.sql](../migrations/20261117000001_cr001_plan_price_history.sql) and [20261117000002_merge_library_schema.sql](../migrations/20261117000002_merge_library_schema.sql) are the source of truth; this page explains them.
**Shared:** storybook_library_app uses this same database and schema and has no migrations of its own ([ADR-010](DECISIONS.md#adr-010)).
**Go structs:** [models/storybook.go](../models/storybook.go)

## Conventions
| Convention | Rule |
|---|---|
| Primary keys | `UUID DEFAULT gen_random_uuid()`. Repositories also set ids in Go before insert |
| Money | `INTEGER` whole IDR. `0` = free. Never float |
| Timestamps | `TIMESTAMPTZ` |
| Enums | `VARCHAR` + `CHECK` (no native PG enums) |
| Soft delete | `books.deleted_at` only. Orders, purchases, and subscriptions are never deleted |
| Business rules in the DB | Anything that must hold under concurrency is a constraint (see "Invariants") |
| New tables/columns | Append a new numbered `.sql` file and add it to `migrations` in [migrations/migration.go](../migrations/migration.go). Never edit an applied file. This includes schema the library app needs |
| Migration order | Filename order **is** apply order: the library app's tests apply `migrations/*.sql` sorted by name. `TestOrdered_MatchesSortedSQLFiles` enforces it |

## Entities
```
credentials ──1:1── users (parents)          credentials ──1:1── admins
credentials ──< refresh_tokens

users ──< book_orders >── books ──1:1── book_files (cover)
users ──< purchases   >── books          book_orders ──1:1── purchases
users ──< subscriptions >── subscription_plans
users ──< subscription_orders >── subscription_plans
subscription_orders ──> subscriptions (set when paid)
subscriptions ──> subscriptions (renewed_from_id)
subscription_plans ──< subscription_plan_price_history >── admins (changed_by)
subscription_plans ──> admins (updated_by)
```

| Table | Purpose | Notable columns |
|---|---|---|
| `credentials` | Login for parents and admins | `email` (unique, also unique on `lower(email)`, stored lowercase), `password_hash` (bcrypt), `account_type` `parent`/`admin`; exactly one of `user_id`/`admin_id` (CHECK) |
| `users` | Parent profile | `name`, `pin_hash` (bcrypt, nullable), `pin_failed_attempts`, `pin_locked_until`, `password_failed_attempts`, `password_locked_until` (Forgot PIN lockout; written only by the library app today) |
| `admins` | Back-office admin profile | `name` |
| `refresh_tokens` | Revocable sessions | `token_hash` (SHA-256 hex, unique), `expires_at` (+7 days), `revoked_at` |
| `books` | Master book | `age_type` `range`/`plus`, `age_min`, `age_max` (range only), `price`, `status` `draft`/`published`, `epub_content BYTEA`, `deleted_at` |
| `book_files` | Cover image, one per book | `book_id` unique, `mime_type` `image/jpeg`/`image/png`, `content BYTEA` |
| `subscription_plans` | The single monthly plan | `code` (unique, `monthly`), `name`, `price` (> 0), `benefits`, `duration` `'1 month'`, `updated_by` (admin, nullable for the seeded row) |
| `subscription_plan_price_history` | Audit of admin price changes (CR-001-2) | `old_price`, `new_price`, `changed_by` (admin), `changed_at`; written in the same transaction as the price update |
| `subscriptions` | One row per subscription period | `status` `active`/`cancelled`/`expired`, `price_paid`, `started_at`, `expires_at`, `cancelled_at`, `renewed_from_id` |
| `book_orders` | Single-book checkout (Order History) | `price` (snapshot at checkout), `status` `pending`/`paid`/`failed`, `idempotency_key`, `paid_at` |
| `subscription_orders` | New and renewal payments | `order_type` `new`/`renewal`, `price` (snapshot), `status`, `idempotency_key`, `subscription_id` |
| `purchases` | Ownership of a book (My Book) | `price_paid`, `purchased_at`, `book_order_id` (unique) |

## Invariants enforced by the database
| Rule | Constraint |
|---|---|
| At most one active subscription per user (E3) | `UNIQUE INDEX uq_subscriptions_one_active ON subscriptions(user_id) WHERE status = 'active'` |
| A book is bought once per user (E1) | `UNIQUE (user_id, book_id)` on `purchases` |
| Double-tap protection (E8) | `UNIQUE (user_id, idempotency_key)` on both order tables; renewals use key `renewal:<previous subscription id>` |
| Age range valid (E16) | `CHECK` range ⇒ `age_max` set and `age_min < age_max`; plus ⇒ `age_max` NULL |
| Prices sane | `CHECK (price >= 0)` on books/orders; `CHECK (price > 0)` on the plan |
| A history row is a real change (E-CR6) | `CHECK (new_price <> old_price)` on `subscription_plan_price_history` |
| Account type matches owner | `CHECK` on `credentials` |
| One account per email, ignoring case | `UNIQUE INDEX uq_credentials_email_lower ON credentials (lower(email))` |

## Who writes what (ADR-011)
| Tables | Written by |
|---|---|
| `books`, `book_files`, `subscription_plans`, `subscription_plan_price_history`, `admins` | this app (back office) |
| `users` (profile, PIN, lockouts), `book_orders`, `purchases`, `subscription_orders`, `subscriptions` | storybook_library_app (parent app and its renewal job) |
| `credentials`, `refresh_tokens` | both: each app logs in, refreshes and logs out only its own account type |

State transitions (`pending → paid/failed`, `active → cancelled/expired`), all made by storybook_library_app, use conditional updates (`UPDATE … WHERE status = 'pending' RETURNING id`). If two requests race, only one wins; the other gets `DUPLICATE_REQUEST`, or is skipped in the renewal job.

## Access rule (derived, not stored)
Applied by storybook_library_app. For a user and a book (PRD §12.1):
1. Book missing, or unpublished/deleted **and not purchased** → `BOOK_NOT_FOUND`
2. `price = 0` → `free`
3. Row in `purchases` → `owned`
4. Row in `subscriptions` with `status = 'active'` → `subscribed`
5. Otherwise → `locked` (content returns `403 BOOK_LOCKED`)

"Active" deliberately ignores `expires_at`: renewal always succeeds (mock), so the hourly job turns ended periods into the next period.

## Query rules
- `epub_content` is read only by storybook_library_app's content endpoint, after the access check passes. This app writes it on upload and never selects it: every book query here uses an explicit column list without it.
- Covers live in `book_files` so book lists never touch image bytes.

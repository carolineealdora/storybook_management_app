# Data Models — Storybook Library
**Last updated:** 2026-11-16
**Schema:** [migrations/20261116000001_create_storybook_schema.sql](../migrations/20261116000001_create_storybook_schema.sql) is the source of truth; this page explains it.
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
| New tables/columns | Append a new numbered `.sql` file and add it to `migrations` in [migrations/migration.go](../migrations/migration.go). Never edit an applied file |

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
```

| Table | Purpose | Notable columns |
|---|---|---|
| `credentials` | Login for parents and admins | `email` (unique, stored lowercase), `password_hash` (bcrypt), `account_type` `parent`/`admin`; exactly one of `user_id`/`admin_id` (CHECK) |
| `users` | Parent profile | `name`, `pin_hash` (bcrypt, nullable), `pin_failed_attempts`, `pin_locked_until` |
| `admins` | Back-office admin profile | `name` |
| `refresh_tokens` | Revocable sessions | `token_hash` (SHA-256 hex, unique), `expires_at` (+7 days), `revoked_at` |
| `books` | Master book | `age_type` `range`/`plus`, `age_min`, `age_max` (range only), `price`, `status` `draft`/`published`, `epub_content BYTEA`, `deleted_at` |
| `book_files` | Cover image, one per book | `book_id` unique, `mime_type` `image/jpeg`/`image/png`, `content BYTEA` |
| `subscription_plans` | The single monthly plan | `price` (> 0), `benefits`, `duration` `'1 month'` |
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
| Account type matches owner | `CHECK` on `credentials` |

State transitions (`pending → paid/failed`, `active → cancelled/expired`) use conditional updates (`UPDATE … WHERE status = 'pending' RETURNING id`). If two requests race, only one wins; the other gets `DUPLICATE_REQUEST`, or is skipped in the renewal job.

## Access rule (derived, not stored)
For a user and a book (PRD §12.1):
1. Book missing, or unpublished/deleted **and not purchased** → `BOOK_NOT_FOUND`
2. `price = 0` → `free`
3. Row in `purchases` → `owned`
4. Row in `subscriptions` with `status = 'active'` → `subscribed`
5. Otherwise → `locked` (content returns `403 BOOK_LOCKED`)

"Active" deliberately ignores `expires_at`: renewal always succeeds (mock), so the hourly job turns ended periods into the next period.

## Query rules
- `epub_content` is selected **only** by `GetWithContent`, used by `GET /books/:id/content` after the access check passes. Every list and detail query uses an explicit column list without it.
- Covers live in `book_files` so book lists never touch image bytes.

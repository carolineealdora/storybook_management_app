# API Contracts — Storybook Library
**Base URL:** `/api/v1` (local: `http://localhost:9999/api/v1`)
**Last updated:** 2026-11-16
**Source of truth for:** the Digital Storybook Library app (parent/child) and the Book Management app (admin).
Behaviour follows [PRD_storybook-lib-app.md](PRD_storybook-lib-app.md); user story (US-) and edge case (E-) ids are referenced inline.

Update this file whenever an endpoint's request, response, or error codes change.

---

## Conventions

### Auth
Send `Authorization: Bearer <access_token>` on every endpoint except those marked **Public**.

| Token | Lifetime | Notes |
|---|---|---|
| Access token (JWT, HS256) | 30 min | Claims: `sub` (credential id), `account_type` (`parent`/`admin`), `user_id` or `admin_id` |
| Refresh token (opaque) | 7 days | Stored hashed server-side; revoked on logout |

When a request returns `401 TOKEN_EXPIRED`, call `POST /auth/refresh` and retry. `401 REFRESH_TOKEN_EXPIRED` or `401 UNAUTHORIZED` from refresh means the user goes back to the Login page (E20, E21).

**Roles:** "Parent" endpoints reject admin tokens and "Admin" endpoints reject parent tokens with `403 FORBIDDEN` (E17). Child view uses the parent's token; hiding prices, payment, and Settings in Child view is the client's job (PLAN D7).

### Envelopes
Success, single resource:
```json
{ "data": { } }
```
Success, list (no pagination; every list returns all rows):
```json
{ "data": [ ], "meta": { "total": 3 } }
```
Error (all errors):
```json
{ "error": { "code": "BOOK_LOCKED", "message": "Subscribe or purchase this book to read it." } }
```
`message` is safe to show to the user. For `VALIDATION_ERROR` it names the failing field, e.g. `"pin must be exactly 4 characters long."`

Binary endpoints (`/books/:id/cover`, `/books/:id/content`) return raw bytes on success and the JSON error envelope on failure.

Every response carries an `X-Request-Id` header. Send your own to correlate logs.

### Money, ids, times
- Prices are **whole IDR** integers (`15000` = Rp 15.000). `0` = free.
- Ids are UUIDs. A malformed id in the path returns `400 VALIDATION_ERROR`.
- Times are RFC 3339 (UTC).

### Error codes
| Code | HTTP | When |
|---|---|---|
| `VALIDATION_ERROR` | 400 | Missing/malformed field, bad JSON, non-UUID id |
| `INVALID_FILE_FORMAT` | 400 | Cover not .jpg/.jpeg/.png or book file not .epub (E15) |
| `INVALID_AGE_RANGE` | 400 | Age not a whole number, or min ≥ max (E16) |
| `UNAUTHORIZED` | 401 | No or invalid token, or the refresh token is unknown or revoked (E19) |
| `TOKEN_EXPIRED` | 401 | Access token expired: refresh and retry (E20) |
| `REFRESH_TOKEN_EXPIRED` | 401 | Refresh token older than 7 days (E21) |
| `INVALID_CREDENTIALS` | 401 | Login email or password wrong (US-01) |
| `INVALID_PIN` | 401 | Wrong PIN (E12) |
| `INVALID_PASSWORD` | 401 | Wrong account password on Forgot PIN / Change PIN (E13) |
| `PAYMENT_FAILED` | 402 | Mock payment returned failed (E9) |
| `FORBIDDEN` | 403 | Wrong account type for this app/endpoint (US-15, E17) |
| `BOOK_LOCKED` | 403 | Content requested without access (E4, E18) |
| `BOOK_NOT_FOUND` | 404 | Book missing, unpublished, or deleted (E22) |
| `ORDER_NOT_FOUND` | 404 | Order missing or belongs to another user |
| `PLAN_NOT_FOUND` | 404 | Subscription plan not configured yet |
| `NOT_FOUND` | 404 | Unknown route |
| `BOOK_ALREADY_OWNED` | 409 | Buying a purchased book (E1) |
| `SUBSCRIPTION_ACTIVE` | 409 | Buying while subscribed (E2), subscribing twice (E3) |
| `BOOK_UNAVAILABLE` | 409 | Book unpublished/deleted between checkout and pay (E6) |
| `DUPLICATE_REQUEST` | 409 | Idempotency key reused, or the order was already paid or failed (E8) |
| `BOOK_NOT_PURCHASABLE` | 409 | Buying a free book (E10) |
| `PIN_NOT_SET` | 409 | Verifying a PIN before one is set (E11) |
| `SUBSCRIPTION_NOT_ACTIVE` | 409 | Cancelling with no active subscription (E14) |
| `PAYLOAD_TOO_LARGE` | 413 | Cover > 5 MB, EPUB > 50 MB, or request > 60 MB |
| `TOO_MANY_ATTEMPTS` | 429 | 5 wrong PINs → locked for 1 minute (E12) |
| `GENERAL_ERROR` | 500 | Unexpected; details are logged, never returned |

---

## Shared response shapes

**BookCard** (showcase item)
```json
{
  "id": "uuid", "title": "The Hungry Cat", "author": "Ana",
  "age_type": "range", "age_min": 1, "age_max": 3, "age_label": "1–3",
  "price": 15000, "is_free": false,
  "access": "free | owned | subscribed | locked",
  "cover_url": "/api/v1/books/{id}/cover"
}
```
`access` drives the UI. Show the lock icon only for `locked`, and the "Free" label for `is_free`. In Child view, hide `price`.

**SubscriptionView**
```json
{
  "id": "uuid", "status": "active | cancelled | expired", "price_paid": 49000,
  "started_at": "…", "expires_at": "… (next renewal date while active)",
  "cancelled_at": null, "renewed_from_id": null,
  "order": { "id": "uuid", "plan_id": "uuid", "order_type": "new | renewal", "price": 49000,
             "status": "paid", "created_at": "…", "paid_at": "…" }
}
```

**BookOrderView**
```json
{ "id": "uuid", "book_id": "uuid", "book_title": "The Hungry Cat", "price": 15000,
  "status": "pending | paid | failed", "created_at": "…", "paid_at": null }
```
`price` is the price shown at checkout. It never changes after the admin edits the book (E7).

---

## Digital Storybook Library app

### POST /auth/login
Auth: Public. Parent accounts only. An admin account gets `403 FORBIDDEN`.
```json
{ "email": "parent@example.com", "password": "…" }
```
`200`
```json
{ "data": { "access_token": "…", "refresh_token": "…", "token_type": "Bearer", "expires_in": 1800, "account_type": "parent" } }
```
Errors: `VALIDATION_ERROR`, `INVALID_CREDENTIALS`, `FORBIDDEN`

### POST /auth/refresh
Auth: Public. Works for parent and admin refresh tokens.
```json
{ "refresh_token": "…" }
```
`200`: same as login, without `refresh_token`. Errors: `UNAUTHORIZED`, `REFRESH_TOKEN_EXPIRED`

### POST /auth/logout
Auth: Parent. Body `{ "refresh_token": "…" }`. `204`. Revokes that refresh token; repeating it is a no-op.

### GET /me
Auth: Parent. Profile, subscription badge, and whether a PIN is set.
```json
{ "data": { "id": "uuid", "name": "Bunda", "email": "parent@example.com", "pin_set": true,
            "is_subscriber": true, "subscription": { /* SubscriptionView without order, or null */ } } }
```

### PATCH /me
Auth: Parent. `{ "name": "Bunda" }` (required, not blank, ≤ 200). `200` → updated profile.

### PUT /me/pin
Auth: Parent. Sets the PIN. Changing an existing PIN requires `password` (US-14).
```json
{ "pin": "1234", "confirm_pin": "1234", "password": "… (only when a PIN already exists)" }
```
`204`. Errors: `VALIDATION_ERROR` (not 4 digits, mismatch, missing password), `INVALID_PASSWORD`

### POST /me/pin/verify
Auth: Parent. Child → Parent switch. `{ "pin": "1234" }`. `204`.
Errors: `PIN_NOT_SET` (409), `INVALID_PIN` (401), `TOO_MANY_ATTEMPTS` (429, the 5th wrong PIN and every attempt during the 1-minute lock)

### POST /me/pin/forgot
Auth: Parent. `{ "password": "…" }`. `204` → switch to Parent view; also clears the PIN lock. Errors: `INVALID_PASSWORD`

### GET /books
Auth: Parent. Showcase: all published books, newest first, as `BookCard[]` with `meta.total`.

### GET /books/:id
Auth: Parent. `BookCard` + `"description"`. A purchased book stays visible to its owner even after it is unpublished or deleted (E5).
Errors: `BOOK_NOT_FOUND`

Detail-page buttons (US-06): `owned`/`free` → **Read**. `subscribed` → disabled **Subscribed** + **Read**. `locked` → **Buy Now** + subscription prompt.

### GET /books/:id/cover
Auth: Parent **or** Admin. Raw image (`image/png` or `image/jpeg`), `Cache-Control: private, max-age=3600`.
Because it needs the Authorization header, load it with `fetch` → `blob` → object URL (a plain `<img src>` cannot send the header).

### GET /books/:id/content
Auth: Parent. Raw EPUB (`application/epub+zip`, `Cache-Control: no-store`) when `access` is not `locked`.
Errors: `BOOK_LOCKED` (403), `BOOK_NOT_FOUND`

### GET /me/books
Auth: Parent. My Book: purchased books only, newest purchase first, including books the admin later deleted (US-08, E5).
`BookCard` + `"price_paid"`, `"purchased_at"`; `access` is always `owned`. Empty list → empty state.

### POST /book-orders
Auth: Parent. Buy Now (one book per order). Generate a fresh `idempotency_key` (e.g. a UUID) per checkout attempt.
```json
{ "book_id": "uuid", "idempotency_key": "uuid" }
```
`201` → `BookOrderView` with `status: "pending"` and the checkout price.
Errors: `BOOK_NOT_FOUND`, `BOOK_NOT_PURCHASABLE`, `BOOK_ALREADY_OWNED`, `SUBSCRIPTION_ACTIVE`, `DUPLICATE_REQUEST`

### POST /book-orders/:id/pay
Auth: Parent. Mocked payment.
```json
{ "result": "success | failed" }
```
`200` → `BookOrderView` with `status: "paid"`; the book is unlocked immediately.
Errors: `PAYMENT_FAILED` (402, order is now `failed`; to retry, create a **new** order with a new key), `DUPLICATE_REQUEST` (already paid or failed: double tap), `BOOK_UNAVAILABLE`, `BOOK_ALREADY_OWNED`, `SUBSCRIPTION_ACTIVE`, `ORDER_NOT_FOUND`

### GET /book-orders
Auth: Parent. Order History: book orders only (paid and failed), newest first, `BookOrderView[]`.

### GET /subscription/plan
Auth: Parent.
```json
{ "data": { "id": "uuid", "price": 49000, "benefits": "Read all published books.", "duration": "1 month", "updated_at": "…" } }
```
Errors: `PLAN_NOT_FOUND`

### POST /subscription-orders
Auth: Parent. `{ "idempotency_key": "uuid" }`. `201`
```json
{ "data": { "id": "uuid", "plan_id": "uuid", "order_type": "new", "price": 49000, "status": "pending", "created_at": "…", "paid_at": null } }
```
Errors: `SUBSCRIPTION_ACTIVE`, `PLAN_NOT_FOUND`, `DUPLICATE_REQUEST`

### POST /subscription-orders/:id/pay
Auth: Parent. `{ "result": "success | failed" }`. `200` → `SubscriptionView` (`active`, `expires_at` = start + 1 month).
Errors: `PAYMENT_FAILED` (no subscription created; retry with a new order), `SUBSCRIPTION_ACTIVE`, `DUPLICATE_REQUEST`, `ORDER_NOT_FOUND`

Auto-renew: an hourly server job renews every active period that has ended. It creates a new `active` period at the current plan price, sets the old one to `expired`, and records a paid `renewal` order. The mock payment always succeeds.

### POST /subscriptions/current/cancel
Auth: Parent. No body. `200` → `SubscriptionView` with `status: "cancelled"`. Access to non-owned books stops immediately, and no refund is given.
Errors: `SUBSCRIPTION_NOT_ACTIVE`

### GET /subscriptions
Auth: Parent. Subscription History: every period with the order that paid for it, newest first, `SubscriptionView[]`.

---

## Book Management app (admin)

### POST /admin/auth/login
Auth: Public. Same body and response as `/auth/login`, with `account_type: "admin"`. A parent account gets `403 FORBIDDEN`.

### POST /admin/auth/logout
Auth: Admin. `{ "refresh_token": "…" }` → `204`

### GET /admin/books
Auth: Admin. Query: `search` (title, case-insensitive), `status` (`draft` | `published`). Excludes deleted books. `AdminBook[]` + `meta.total`.

**AdminBook**
```json
{ "id": "uuid", "title": "…", "author": "…", "description": "…",
  "age_type": "range | plus", "age_min": 1, "age_max": 3, "age_label": "1–3",
  "price": 15000, "status": "draft | published", "cover_url": "/api/v1/books/{id}/cover",
  "epub_file_name": "book.epub", "epub_size_bytes": 1048576, "created_at": "…", "updated_at": "…" }
```

### GET /admin/books/:id
Auth: Admin. `AdminBook` (drafts included). Errors: `BOOK_NOT_FOUND` (missing or deleted)

### POST /admin/books
Auth: Admin. `multipart/form-data`:

| Field | Rule |
|---|---|
| `title`, `author`, `description` | required |
| `age_type` | `range` or `plus` |
| `age_min` | whole number ≥ 0 |
| `age_max` | `range`: whole number > `age_min`; `plus`: omit |
| `price` | whole number ≥ 0 (0 = free) |
| `status` | `draft` or `published` |
| `cover` | file, .jpg/.jpeg/.png, ≤ 5 MB, content must be JPEG/PNG |
| `epub` | file, .epub, ≤ 50 MB, must be an EPUB (ZIP with `application/epub+zip` mimetype) |

`201` → `AdminBook`. Errors: `VALIDATION_ERROR`, `INVALID_AGE_RANGE`, `INVALID_FILE_FORMAT`, `PAYLOAD_TOO_LARGE`

### PUT /admin/books/:id
Auth: Admin. Same form; all text fields required. `cover` and `epub` are **optional**: omit them to keep the stored files. A new price applies to new orders only. `200` → `AdminBook`.

### DELETE /admin/books/:id
Auth: Admin. Soft delete → `204`. The book disappears from the showcase and admin list; owners keep it in My Book (E5). Errors: `BOOK_NOT_FOUND`

### GET /admin/subscription-plan
Auth: Admin. Same shape as `GET /subscription/plan`.

### PUT /admin/subscription-plan
Auth: Admin.
```json
{ "price": 59000, "benefits": "Read all published books." }
```
`price` whole number > 0, `benefits` required. `200` → plan. Applies to new subscriptions and renewals; active periods keep their `price_paid` (US-17).

---

## Other

### GET /healthcheck
Auth: Public. `{ "data": { "database": "UP | DOWN" } }`

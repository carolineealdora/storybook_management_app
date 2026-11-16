# API Contracts — Storybook Back Office
**Base URL:** `/api/v1` (local: `http://localhost:9999/api/v1`)
**Last updated:** 2026-11-16
**Source of truth for:** the Book Management app (`fe_storybook_management_app`, admin only).
**Parent/child endpoints** are served by storybook_library_app; see that repo's `docs/API-contracts.md` ([ADR-011](DECISIONS.md#adr-011)).
Behaviour follows [PRD_storybook-lib-app.md](PRD_storybook-lib-app.md) and [CR-001](CR-001_Badge_and_Subscription_Pricing.md); user story (US-), edge case (E-, E-CR) ids are referenced inline.

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

**Roles:** every protected endpoint here requires an admin token; a parent token gets `403 FORBIDDEN` (E17).

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

The binary endpoint (`/books/:id/cover`) returns raw bytes on success and the JSON error envelope on failure.

Every response carries an `X-Request-Id` header. Send your own to correlate logs.

### CORS
Same-origin only by default: the back office is served on the same origin as `/api` (Vite proxy in development, nginx in production), so no `Access-Control-*` headers are sent. A deployment that serves a frontend from another origin lists it in `CORS_ALLOW_ORIGINS` (comma separated).

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
| `INVALID_PRICE` | 400 | Subscription plan price ≤ 0 (E-CR2) |
| `UNAUTHORIZED` | 401 | No or invalid token, or the refresh token is unknown or revoked (E19) |
| `TOKEN_EXPIRED` | 401 | Access token expired: refresh and retry (E20) |
| `REFRESH_TOKEN_EXPIRED` | 401 | Refresh token older than 7 days (E21) |
| `INVALID_CREDENTIALS` | 401 | Login email or password wrong |
| `FORBIDDEN` | 403 | Wrong account type for this app/endpoint (US-15, E17) |
| `BOOK_NOT_FOUND` | 404 | Book missing, unpublished, or deleted (E22) |
| `PLAN_NOT_FOUND` | 404 | Subscription plan not configured yet |
| `NOT_FOUND` | 404 | Unknown route |
| `PAYLOAD_TOO_LARGE` | 413 | Cover > 5 MB, EPUB > 50 MB, or request > 60 MB |
| `GENERAL_ERROR` | 500 | Unexpected; details are logged, never returned |

---

## Auth (shared by the back office)

### POST /auth/refresh
Auth: Public. **Admin refresh tokens only.** The `refresh_tokens` table is shared with storybook_library_app, so a parent refresh token gets `401 UNAUTHORIZED`.
```json
{ "refresh_token": "…" }
```
`200`: same as admin login, without `refresh_token`. Errors: `UNAUTHORIZED`, `REFRESH_TOKEN_EXPIRED`

### GET /books/:id/cover
Auth: Admin. Raw image (`image/png` or `image/jpeg`), `Cache-Control: private, max-age=3600`. Works for drafts. The path has no `/admin` prefix for frontend compatibility.
Because it needs the Authorization header, load it with `fetch` → `blob` → object URL (a plain `<img src>` cannot send the header).
Errors: `BOOK_NOT_FOUND`

---

## Book Management app (admin)

### POST /admin/auth/login
Auth: Public. A parent account gets `403 FORBIDDEN`.
```json
{ "email": "admin@example.com", "password": "…" }
```
`200`
```json
{ "data": { "access_token": "…", "refresh_token": "…", "token_type": "Bearer", "expires_in": 1800, "account_type": "admin" } }
```
Errors: `VALIDATION_ERROR`, `INVALID_CREDENTIALS`, `FORBIDDEN`

### POST /admin/auth/logout
Auth: Admin. `{ "refresh_token": "…" }` → `204`

### GET /admin/books
Auth: Admin. Query: `search` (title, case-insensitive), `status` (`draft` | `published`). Excludes deleted books. `AdminBook[]` + `meta.total`.

**AdminBook**
```json
{ "id": "uuid", "title": "…", "author": "…", "description": "…",
  "age_type": "range | plus", "age_min": 1, "age_max": 3, "age_label": "1–3",
  "price": 15000, "status": "draft | published", "cover_url": "/api/v1/books/{id}/cover",
  "epub_file_name": "book.epub", "epub_size_bytes": 1048576, "created_at": "…", "updated_at": "…",
  "purchase_count": 38 }
```
`purchase_count` is how many users own the book (rows in `purchases`). It is `0` when nobody has bought it, and always `0` for a book just created. The delete confirmation uses it: "38 families who bought it keep access in My Book" (E5).

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
Auth: Admin. **AdminPlan**: the plan, who last edited it, and its price history (newest first).
```json
{ "data": { "id": "uuid", "code": "monthly", "name": "Monthly Subscription", "price": 59000,
            "benefits": "…", "duration": "1 month", "updated_at": "…", "updated_by": "admin uuid | null",
            "active_subscriber_count": 214,
            "price_history": [ { "id": "uuid", "old_price": 49000, "new_price": 59000,
                                 "changed_by": "admin uuid", "changed_by_name": "Demo Admin | null",
                                 "changed_at": "…" } ] } }
```
- `active_subscriber_count`: subscriptions with `status = active` and `expires_at` in the future, one per user (the same rule as the Subscribed badge, CR-001-1 B1).
- `changed_by_name`: the editing admin's name; `null` only if that admin no longer exists.

Errors: `PLAN_NOT_FOUND`

### PUT /admin/subscription-plan
Auth: Admin.
```json
{ "price": 59000, "benefits": "Read all published books." }
```
`200` → AdminPlan.
- `price` must be a whole number > 0, otherwise `400 INVALID_PRICE` (E-CR2). A non-number is `400 VALIDATION_ERROR`. `benefits` is required.
- New subscribers pay the new price immediately. Active subscribers keep their `price_paid` until their next renewal, which charges the new price (US-17, CR-001-2).
- A price change writes a `price_history` row. The same price writes no row (E-CR6); the same price and benefits writes nothing.
- Before saving, the back-office confirms: "New subscribers will pay Rp [new] immediately. Existing subscribers will pay Rp [new] from their next renewal."
- Errors: `INVALID_PRICE`, `VALIDATION_ERROR`, `FORBIDDEN` (parent token, E-CR3)

---

## Other

### GET /healthcheck
Auth: Public. `{ "data": { "database": "UP | DOWN" } }`

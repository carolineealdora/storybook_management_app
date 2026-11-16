# Frontend Generation Prompts

Two prompts, one per frontend (ARCHITECTURE.md: two Lovable apps, one Go API). Paste each into a **separate** Lovable project (or Claude Code / v0 in an empty repo). Each prompt stands on its own.

Before you run them:
- The backend's `CORS_ALLOW_ORIGINS` only allows `http://localhost:5173`. Add the Lovable preview/deploy origins to it, or every call fails in the browser.
- The prompts target the API as it is **built today** ([API-contracts.md](API-contracts.md)). CR-001 changes only the badge wording here. Its price-change rules (`PRICE_CHANGED`, price history, `next_renewal_price`) are not in the API yet, so the prompts leave them out.

---

## Prompt A — Digital Storybook Library app (parent + child)

```text
Build "MainStory Library", a mobile- and tablet-first web app where parents and young children (1–6 years) browse, buy, subscribe to, and read EPUB picture books. It talks to an existing REST API. Do NOT build a backend, do NOT use Supabase, and do NOT mock data: every screen reads from the API below.

## Stack
React + Vite + TypeScript, Tailwind + shadcn/ui, React Router, TanStack Query, epub.js (for the reader), lucide-react icons.
API base URL from `import.meta.env.VITE_API_BASE_URL` (default `http://localhost:9999/api/v1`).

## Look & feel
Warm, playful, and soft: rounded cards, big colorful covers, friendly rounded font (e.g. Nunito). Touch targets at least 48×48 px. Works in portrait and landscape. Every screen has loading (skeletons), empty, success, and error states. Prices are whole IDR: format with `new Intl.NumberFormat('id-ID', { style: 'currency', currency: 'IDR', maximumFractionDigits: 0 })` → "Rp 15.000". Price 0 shows "Free".

## API conventions
- Success: `{ "data": ... }`; lists: `{ "data": [...], "meta": { "total": n } }`. No pagination.
- Error: `{ "error": { "code": "BOOK_LOCKED", "message": "..." } }`. `message` is safe to show the user. Branch on `code`, never on HTTP text.
- Send `Authorization: Bearer <access_token>` on every call except login/refresh.
- Ids are UUIDs, times are RFC 3339 UTC. Show dates in the user's locale.

## Auth (one API client module)
- `POST /auth/login` `{email, password}` → `{access_token, refresh_token, token_type, expires_in, account_type}`. Store both tokens in localStorage.
- On `401 TOKEN_EXPIRED`: call `POST /auth/refresh` `{refresh_token}` → new `access_token`, then retry the original request once. If several requests fail at once, run only ONE refresh and queue the rest.
- On `401 UNAUTHORIZED` or `REFRESH_TOKEN_EXPIRED` (from any call, including refresh): clear tokens and go to /login.
- Login errors: `INVALID_CREDENTIALS` → "Email or password is incorrect". `FORBIDDEN` → "This account cannot use the library app" (it is an admin account).
- No sign-up or forgot-password links.
- Logout: `POST /auth/logout` `{refresh_token}` (204), then clear tokens and go to /login. Clear tokens even if the call fails.

## Images and book files need the auth header
`GET /books/:id/cover` and `GET /books/:id/content` require the Bearer header, so a plain `<img src>` will not work. Build a `<Cover bookId>` component: fetch with the header → blob → `URL.createObjectURL`, revoke it on unmount, cache with TanStack Query, and show a soft placeholder while loading or on error.

## View mode: Parent and Child (client-side only)
One account and one token. The mode is stored in localStorage (`view_mode`) and survives reloads. After login, always start in Parent view.
- Header (both views): account badge top-left, the view toggle top-right. In Parent view only, a "Subscribe" button sits next to the toggle, shown ONLY when the user is not subscribed.
- Badge: "Subscribed" when `GET /me` → `is_subscriber` is true, otherwise "Free". Do not use "Premium", "Member", or "Non-subscriber".
- Parent → Child: if `me.pin_set` is false, show a dialog "Set a PIN in Settings first" with a button to Settings, and stay in Parent view. If it is true, switch immediately with no PIN.
- Child → Parent: open a 4-digit PIN pad dialog → `POST /me/pin/verify {pin}` (204 = switch to Parent). Errors: `INVALID_PIN` → "Wrong PIN, try again". `TOO_MANY_ATTEMPTS` → disable the PIN pad and show a 60-second countdown. `PIN_NOT_SET` → treat as Parent view.
- The PIN dialog has a "Forgot PIN?" link → password field → `POST /me/pin/forgot {password}` (204 = switch to Parent and go to Settings so they can set a new PIN). `INVALID_PASSWORD` → show the error and stay in Child view.
- Child view hides: prices, descriptions, Buy Now, the subscription prompt, the Subscribe button, checkout, payment, and Settings. Footer: Home, My Book. Parent view footer: Home, My Book (middle), Settings.
- Route guard: in Child view, any parent-only route (book detail, checkout, subscription, settings, histories) redirects to Home.

## Book card (shared)
`GET /books` returns BookCard[]:
`{ id, title, author, age_type: "range"|"plus", age_min, age_max, age_label, price, is_free, access: "free"|"owned"|"subscribed"|"locked", cover_url }`
- Cover fills the card. `age_label` chip at the TOP-LEFT of the cover. A lock icon at the BOTTOM-RIGHT of the cover only when `access === "locked"`. Free books show a "Free" label and never a lock.
- Below the cover: title, and in Parent view the price ("Free" if `is_free`). Never put a price tag on the cover.
- Drive all lock/unlock UI from `access`. Do not recompute it on the client.

## Screens (Parent view unless noted)
1. **/login**: email + password.
2. **Home /**: grid of all books from `GET /books` (newest first, one mixed showcase, no sections). 2 columns on phones, 3–4 on tablets.
   - Parent: tapping a card opens the detail page.
   - Child: tapping an unlocked card opens the reader. Tapping a locked card shows a friendly, price-free message: "Ask your parent to unlock this book 🔒".
3. **Book detail /books/:id** (`GET /books/:id`, which adds `description`): cover, title, author, age range, description, price. Buttons by `access`:
   - `free` or `owned` → **Read**.
   - `subscribed` → disabled **Subscribed** button + **Read**.
   - `locked` → **Buy Now** + a subscription prompt card: "Read all books for Rp X/month" (price from `GET /subscription/plan`) linking to /subscription.
   - `BOOK_NOT_FOUND` → not-found state with a button back to Home.
4. **Checkout /checkout/:bookId**: one book only (cover, title, price); there is no cart.
   - On entering, generate `idempotency_key = crypto.randomUUID()` and call `POST /book-orders {book_id, idempotency_key}` → 201 BookOrderView `{id, book_id, book_title, price, status, created_at, paid_at}`. Show `order.price` (the checkout snapshot), not the live book price.
   - Mock payment panel with two buttons: **Pay** (sends `{result:"success"}`) and **Simulate failed payment** (sends `{result:"failed"}`) → `POST /book-orders/:id/pay`.
   - Disable both buttons while the request is in flight (double-tap guard).
   - 200 → success screen: "Book unlocked!" with a **Read Now** button. Invalidate the `books`, `book`, `me/books`, and `book-orders` queries.
   - `PAYMENT_FAILED` (402): show "Payment failed, the book is still locked" and a **Try again** button. Try again creates a NEW order with a NEW idempotency key, because a failed order cannot be paid again.
   - `BOOK_UNAVAILABLE`: toast "This book is no longer available" and go to Home.
   - `BOOK_ALREADY_OWNED`: go to the reader. `SUBSCRIPTION_ACTIVE`: go back to the detail page. `BOOK_NOT_PURCHASABLE`: go back to the detail page. `DUPLICATE_REQUEST`: refetch the order state and do not show an error.
5. **Reader /read/:id** (both views): fetch `GET /books/:id/content` with the auth header as an ArrayBuffer and open it with `ePub(arrayBuffer)`.
   - Paginated rendition, no scrolling. Big **Back** / **Next** buttons. Back is disabled on the first page and Next on the last (use the `relocated` event: `location.atStart` / `location.atEnd`).
   - Day/Night toggle via `rendition.themes` (light background vs dark background with light text) that does not leave the page.
   - Close button returns to the previous page. Fits portrait and landscape; re-render on resize.
   - `BOOK_LOCKED` (403) → friendly "This book is locked" screen (no price in Child view; in Parent view, link to the detail page). Never offer a download.
6. **My Book /my-books** (both views): `GET /me/books` → BookCard + `price_paid`, `purchased_at`, newest first. Tap → reader. Child view hides prices. Empty state: "No books yet" + a button to Home.
7. **Subscription /subscription**: load `GET /me` and `GET /subscription/plan` `{id, price, benefits, duration, updated_at}`.
   - Not subscribed: show benefits, "Rp X / month", and **Checkout** → `POST /subscription-orders {idempotency_key}` → then the same mock payment panel → `POST /subscription-orders/:id/pay {result}` → 200 SubscriptionView.
   - On success: the badge changes to "Subscribed" and every book unlocks WITHOUT a page reload (invalidate `me`, `books`, `book`, `subscriptions`).
   - `PAYMENT_FAILED`: "Payment failed, no subscription was created" + retry (new order, new key). `PLAN_NOT_FOUND`: "Subscription is not available yet".
   - Subscribed: show the current plan, `price_paid`, the next renewal date (`expires_at`, "renews automatically"), and a **Cancel subscription** button.
   - Cancel opens a confirm dialog: "Access to books you didn't buy stops immediately. No refund is given. Books you bought stay in My Book." Buttons: "Keep subscription" / "Cancel subscription" → `POST /subscriptions/current/cancel`. Then invalidate as above; the badge shows "Free" and the Subscribe button reappears. `SUBSCRIPTION_NOT_ACTIVE` → refetch.
8. **Settings /settings** (Parent only):
   - Email shown read-only. Name edit → `PATCH /me {name}` (required, not blank, max 200 characters).
   - PIN form: `pin` and `confirm_pin` (exactly 4 digits, must match). If `me.pin_set` is true, also ask for the account `password` → `PUT /me/pin` (204). `INVALID_PASSWORD` → show it on the password field.
   - Links to Subscription, Order History, Subscription History, and **Logout**.
9. **Order History /orders**: `GET /book-orders` → date, book title, price, status badge (Paid green / Failed red / Pending grey), newest first. Empty state.
10. **Subscription History /subscriptions**: `GET /subscriptions` → SubscriptionView[] `{id, status: "active"|"cancelled"|"expired", price_paid, started_at, expires_at, cancelled_at, renewed_from_id, order: {order_type: "new"|"renewal", price, status, paid_at}}`. Show start date, end date, price paid, status, and "Renewal" when `order.order_type === "renewal"`. Empty state.

`GET /me` → `{ id, name, email, pin_set, is_subscriber, subscription: {id, status, price_paid, started_at, expires_at, cancelled_at} | null }`. Use it for the badge, the Subscribe button, and the PIN logic. Refetch it on window focus.

## Error code → UI (central helper)
VALIDATION_ERROR (show message on the form) · INVALID_CREDENTIALS · INVALID_PIN · INVALID_PASSWORD · TOKEN_EXPIRED (silent refresh) · REFRESH_TOKEN_EXPIRED / UNAUTHORIZED (to login) · FORBIDDEN · PAYMENT_FAILED · BOOK_LOCKED · BOOK_NOT_FOUND · ORDER_NOT_FOUND · PLAN_NOT_FOUND · BOOK_ALREADY_OWNED · SUBSCRIPTION_ACTIVE · BOOK_UNAVAILABLE · DUPLICATE_REQUEST · BOOK_NOT_PURCHASABLE · PIN_NOT_SET · SUBSCRIPTION_NOT_ACTIVE · TOO_MANY_ATTEMPTS · GENERAL_ERROR ("Something went wrong, please try again").
In Child view, every error message is friendly and never mentions payment or prices.

## Out of scope (do not build)
Sign-up, forgot password, change email/password, cart, reviews, categories, child profiles, audio, offline reading, book downloads.
```

---

## Prompt B — Book Management app (admin back office)

```text
Build "MainStory Book Management", a desktop-first admin back office for managing a children's EPUB book catalogue and a monthly subscription plan. It talks to an existing REST API. Do NOT build a backend, do NOT use Supabase, and do NOT mock data.

## Stack
React + Vite + TypeScript, Tailwind + shadcn/ui, React Router, TanStack Query, react-hook-form + zod, lucide-react.
API base URL from `import.meta.env.VITE_API_BASE_URL` (default `http://localhost:9999/api/v1`).

## Look & feel
Clean admin UI: left sidebar (Books, Subscription Plan, Logout), data table, form pages, confirm dialogs, and toasts. Loading, empty, and error states everywhere. Prices are whole IDR, shown as "Rp 15.000" via `Intl.NumberFormat('id-ID', {style:'currency', currency:'IDR', maximumFractionDigits:0})`; 0 shows "Free".

## API conventions
- Success `{ "data": ... }`; lists `{ "data": [...], "meta": { "total": n } }` (no pagination).
- Error `{ "error": { "code", "message" } }`. `message` is safe to show. Branch on `code`.
- `Authorization: Bearer <access_token>` on every call except login/refresh.

## Auth
- `/login`: `POST /admin/auth/login {email, password}` → `{access_token, refresh_token, expires_in, account_type:"admin"}`. Store both in localStorage and go to /books.
- `INVALID_CREDENTIALS` → "Email or password is incorrect". `FORBIDDEN` → "This account has no back-office access" (it is a parent account).
- On `401 TOKEN_EXPIRED`: single-flight `POST /auth/refresh {refresh_token}` → retry the request once. On `UNAUTHORIZED` / `REFRESH_TOKEN_EXPIRED`: clear tokens → /login.
- Logout: `POST /admin/auth/logout {refresh_token}` (204), then clear tokens → /login.
- Every route except /login is protected.

## Covers need the auth header
`GET /books/:id/cover` needs the Bearer header, so fetch → blob → `URL.createObjectURL` in a `<Cover bookId>` component (revoke on unmount, show a placeholder on error).

## AdminBook
`{ id, title, author, description, age_type: "range"|"plus", age_min, age_max, age_label, price, status: "draft"|"published", cover_url, epub_file_name, epub_size_bytes, created_at, updated_at }`

## Screens
1. **Books /books**: `GET /admin/books?search=&status=`.
   - Search box (debounced 300 ms, matches title, case-insensitive) + status filter (All / Draft / Published), both kept in the URL query.
   - Table columns: cover thumbnail, title, author, age (`age_label`), price, status badge, updated_at, actions (Edit, Delete). Show the total count.
   - "Add book" button.
   - Delete opens a confirm dialog: "Hide this book from the library? Parents who bought it keep it in My Book." → `DELETE /admin/books/:id` (204) → toast + refetch.
2. **Create /books/new** and **Edit /books/:id/edit** (`GET /admin/books/:id` to prefill). Send as `multipart/form-data` (`POST /admin/books` or `PUT /admin/books/:id`). Fields:
   - `title`, `author`, `description` (textarea): required.
   - Age range: a segmented control **Range** / **Number**, sent as `age_type` = `range` or `plus`.
     - Range: two whole-number inputs "[min] – [max]"; min ≥ 0, and max must be greater than min. Live preview e.g. "1–3".
     - Number: one input (`age_min`), preview e.g. "3+". Do NOT send `age_max`.
   - `price`: whole number ≥ 0, with the hint "0 = free".
   - `status`: Draft / Published radio. Hint: "Only published books appear in the library app."
   - `cover`: accept .jpg, .jpeg, .png, max 5 MB, with an image preview.
   - `epub`: accept .epub, max 50 MB; show the file name and size.
   - Create: both files are required. Edit: both files are optional (omit them to keep the stored files). Show the current cover and the current `epub_file_name` / size.
   - Client-side zod validation that mirrors these rules, but still show server errors: `VALIDATION_ERROR` / `INVALID_AGE_RANGE` / `INVALID_FILE_FORMAT` / `PAYLOAD_TOO_LARGE` → show the `message` on the form.
   - Disable Save and show upload progress while submitting. On success: toast → /books.
   - Edit hint under price: "A new price applies to new orders only."
   - `BOOK_NOT_FOUND` → not-found state with a link back to /books.
3. **Subscription Plan /subscription-plan**: `GET /admin/subscription-plan` → `{id, price, benefits, duration, updated_at}`.
   - `PLAN_NOT_FOUND` → empty form to create it.
   - Form: `price` (whole number > 0), `benefits` (textarea, required); duration shown read-only ("1 month").
   - Save opens a confirm dialog: "New subscribers and renewals will pay Rp [new]. Active subscriptions keep the price they paid until their period ends." → `PUT /admin/subscription-plan {price, benefits}` → toast. Show "Last updated" from `updated_at`.

## Error codes to handle
VALIDATION_ERROR · INVALID_AGE_RANGE · INVALID_FILE_FORMAT · PAYLOAD_TOO_LARGE · UNAUTHORIZED · TOKEN_EXPIRED · REFRESH_TOKEN_EXPIRED · INVALID_CREDENTIALS · FORBIDDEN · BOOK_NOT_FOUND · PLAN_NOT_FOUND · GENERAL_ERROR ("Something went wrong, please try again").

## Out of scope (do not build)
User management, roles, sales dashboards, price history, bulk upload, pagination.
```

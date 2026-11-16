# Tech Debt Registry
Last updated: 2026-11-16

## Priority: Critical (blocks scaling or reliability)
<!-- Empty -->

## Priority: High (significant impact, address in next sprint)
- [ ] **TD-1: Child can call parent-only endpoints with the shared token**
  - Impact: The PIN gate is enforced only in the UI. Anyone holding the parent token can call `/book-orders/*/pay`, `/subscription-orders/*/pay` and `/me/pin` directly. Matters once real payments exist.
  - Source: feat/STORY-1 Phase 3 (ADR-007)
  - Created: 2026-11-16
  - Notes: Issue a short-lived "parent mode" token from `POST /me/pin/verify` and require it on payment and settings endpoints, or add child profiles.
- [ ] **TD-2: Mocked auth and payment**
  - Impact: No sign-up or password reset. The client decides whether a payment succeeds. Not shippable to real parents.
  - Source: PRD assumptions / feat/STORY-1 Phases 2, 6, 7 (ADR-005)
  - Created: 2026-11-16
  - Notes: Replace pay endpoints with gateway initiation plus a signed webhook (HMAC-verified, idempotent on the gateway payment id). The order and transaction code can stay.

## Priority: Medium (worth fixing, not urgent)
- [ ] **TD-3: No rate limit on `/auth/login`, `/admin/auth/login`, `/me/pin/forgot`**
  - Impact: Passwords can be brute-forced. Only PIN verify has a lockout.
  - Source: feat/STORY-1 Phases 2–3
  - Created: 2026-11-16
- [ ] **TD-4: EPUB and cover bytes stored in PostgreSQL**
  - Impact: DB size and backup time grow with the catalogue. Each read loads the whole EPUB into memory (≤ 50 MB per request).
  - Source: PRD §11 decision / feat/STORY-1 Phase 4 (ADR-001)
  - Created: 2026-11-16
  - Notes: Move to object storage with short-lived signed URLs issued after the access check.
- [ ] **TD-5: In-process renewal scheduler**
  - Impact: Every instance scans for due subscriptions (safe, but duplicated work). A renewal is delayed by up to `RENEWAL_INTERVAL` after expiry.
  - Source: feat/STORY-1 Phase 7 (ADR-006)
  - Created: 2026-11-16
  - Notes: Run it as a single scheduled worker if instances multiply. Add a grace/`past_due` state once renewals can fail.
- [ ] **TD-6: Refresh tokens are not rotated**
  - Impact: A stolen refresh token works for up to 7 days unless the user logs out.
  - Source: feat/STORY-1 Phase 2 (ADR-003)
  - Created: 2026-11-16
- [ ] **TD-12: Each backend still exposes the other app's endpoints**
  - Impact: Routing is settled: `fe_storybook_management_app` calls only this backend, and `fe_storybook_library_app` calls only storybook_library_app. But this backend still serves parent login and every parent endpoint, without the library app's parent-mode token or Forgot PIN lockout. A parent can log in here and call `/me/pin/forgot` or the pay endpoints directly, bypassing those controls. The library backend likewise still serves admin endpoints.
  - Source: feat/STORY-3-shared-schema (ADR-010)
  - Created: 2026-11-16
  - Notes: Remove the parent routes here (keep `GET /books/:id/cover` for admins) and the admin routes in storybook_library_app. Auth change, so it needs `SECURITY: manual review required`.
- [ ] **TD-13: Both backends run the hourly renewal job on the shared database**
  - Impact: Duplicate scans; one of the two renewals per period always loses. That is safe thanks to the conditional update and the `renewal:<id>` key, but the loser may log an error (unverified).
  - Source: feat/STORY-3-shared-schema (ADR-010)
  - Created: 2026-11-16
  - Notes: Run it only in storybook_library_app, which owns the parent subscription flows.

## Priority: Low (nice to have)
- [ ] **TD-7: No pagination on list endpoints**
  - Impact: Showcase, admin list and histories return every row. Fine for an MVP catalogue; slow past a few hundred books or orders.
  - Source: feat/STORY-1 (ADR-008)
  - Created: 2026-11-16
- [ ] **TD-8: EPUB validation is a header sniff**
  - Impact: A ZIP starting with the EPUB mimetype but with a broken OPF passes upload, then fails in the reader.
  - Source: feat/STORY-1 Phase 4 (`isEpub`)
  - Created: 2026-11-16
- [ ] **TD-9: Covers need an Authorization header**
  - Impact: The frontend cannot use a plain `<img src>`; it must `fetch` the cover into a blob.
  - Source: feat/STORY-1 Phase 5
  - Created: 2026-11-16
  - Notes: Make covers public, or use signed URLs, if this becomes a frontend burden.
- [ ] **TD-10: Migrator `down` only forgets migrations, never drops tables**
  - Impact: `migrate down` does not undo the schema (inherited scaffold behaviour).
  - Source: scaffold / feat/STORY-1 Phase 0
  - Created: 2026-11-16
- [ ] **TD-11: Subscription pay checks the plan price outside the activation transaction**
  - Impact: If the admin changes the price in the milliseconds between the `PRICE_CHANGED` check and `Activate`, the subscriber is charged the checkout price instead of getting 409. Harmless with mock payment; matters with a real gateway.
  - Source: feat/STORY-2 CR-001 (`PayOrder`)
  - Created: 2026-11-16
  - Notes: Re-check `subscription_plans.price` inside the `Activate` transaction (`SELECT … FOR SHARE`).

## Resolved
<!-- Empty -->

# CR-001 — Account Badge & Subscription Price Change

| Field | Value |
|---|---|
| **Type** | Change Request (addendum to PRD v0.1) |
| **Status** | Ready for implementation |
| **Last updated** | 2026-11-16 |
| **Affects** | Backend API, Customer App, Back-Office App |

> This document **adds to** the existing PRD. Anything not mentioned here stays as defined in the PRD.

---

## 1. Summary of Changes

| # | Change | Apps affected |
|---|---|---|
| CR-001-1 | Account badge shows **"Free"** for non-subscribers and **"Subscribed"** for subscribers | Customer App (+ Back-Office user view, if it exists) |
| CR-001-2 | When the subscription price changes, **existing subscribers keep their current price until their next renewal**. New subscribers pay the new price immediately. | Backend, Back-Office, Customer App |

---

## 2. CR-001-1 — Account Badge

### 2.1 Rules

| # | Condition | Badge |
|---|---|---|
| B1 | User has an active subscription (`status = active` AND `expires_at > now`) | **Subscribed** |
| B2 | User has no subscription | **Free** |
| B3 | User's subscription has expired or been cancelled and the period has ended | **Free** |
| B4 | User has no subscription but owns purchased books | **Free** (purchases do not change the badge) |
| B5 | Admin account | No customer badge (admin UI only) |

### 2.2 Implementation Notes
- The badge is **derived by the backend**, not computed in the frontend.
- `GET /me` returns a new field:
  ```json
  { "plan_status": "subscribed" | "free" }
  ```
- The frontend maps `subscribed` → "Subscribed" and `free` → "Free".
- The badge updates **immediately** after a successful subscription purchase, with no manual refresh.
- Replace any previous badge labels (e.g. "Premium", "Member", "Basic") everywhere in the UI.

### 2.3 Acceptance Criteria
- [ ] Given a user with an active subscription, when they open the app, then the badge shows **"Subscribed"**.
- [ ] Given a user without a subscription, when they open the app, then the badge shows **"Free"**.
- [ ] Given a user who owns purchased books but has no subscription, then the badge shows **"Free"**.
- [ ] Given a user whose subscription expired, when they open the app or refresh, then the badge shows **"Free"**.
- [ ] Given a free user, when the subscription payment succeeds, then the badge changes to **"Subscribed"** without a page reload.

---

## 3. CR-001-2 — Subscription Price Change Applies at Next Renewal

### 3.1 Rules

| # | Scenario | Price applied |
|---|---|---|
| P1 | Admin changes the subscription price | New price is saved as the **current plan price** |
| P2 | **Existing active subscriber** | Keeps the price they paid until `expires_at`. The **new price applies at the next renewal**. |
| P3 | **New subscriber** (never subscribed) after the change | Pays the **new price** immediately |
| P4 | **Expired user re-subscribing** after the change | Pays the **new price** (this counts as a new subscription) |
| P5 | Price changes **multiple times** before a subscriber's renewal | Renewal uses the **latest price at the moment of renewal** |
| P6 | Price **decreases** | Same rule: applies at next renewal; no partial refund for the current period |
| P7 | Price change never affects **one-time book purchases** | — |

### 3.2 Data Model Changes

**New table `subscription_plans`** (single plan for the MVP, but the structure allows more later)

| Field | Type | Notes |
|---|---|---|
| id | uuid / serial | PK |
| code | varchar | e.g. `monthly` |
| name | varchar | e.g. "Monthly Subscription" |
| price | integer | IDR, must be > 0 |
| duration_days | integer | e.g. 30 |
| updated_at | timestamp | |
| updated_by | FK → users.id | Admin who changed it (audit) |

**Optional: new table `subscription_plan_price_history`** (audit trail; recommended)

| Field | Type |
|---|---|
| id | PK |
| plan_id | FK → subscription_plans.id |
| old_price | integer |
| new_price | integer |
| changed_by | FK → users.id |
| changed_at | timestamp |

**Update table `subscriptions`**

| Field | Change |
|---|---|
| plan_id | **Add** — FK → subscription_plans.id |
| price_paid | **Add** — price locked for the current period |
| renewed_at | **Add** (nullable) — last renewal timestamp |

> Remove any hardcoded subscription price from code and config; always read it from `subscription_plans`.

### 3.3 API Changes

| Method | Endpoint | Auth | Description |
|---|---|---|---|
| GET | `/subscription-plan` | User | Returns the current plan price (used on the paywall) |
| PUT | `/admin/subscription-plan` | Admin | Update the plan price (writes a price history row) |
| POST | `/payments/subscription` | User | New subscription: charges the **current plan price** and stores it in `price_paid` |
| POST | `/payments/subscription/renew` | User | **Mock renewal**: charges the **current plan price**, extends `expires_at` by `duration_days`, and updates `price_paid` and `renewed_at` |

**`GET /me` additions for subscribers:**
```json
{
  "plan_status": "subscribed",
  "subscription": {
    "price_paid": 49000,
    "expires_at": "2026-12-16T00:00:00Z",
    "next_renewal_price": 59000,
    "price_change_pending": true
  }
}
```

**Price-mismatch protection:** the client sends `expected_price` in payment requests. If it does not match the current plan price (because the admin changed it while the user was on the paywall), return:
```json
{ "error": { "code": "PRICE_CHANGED", "message": "The subscription price has changed. Please review the new price." } }
```
with HTTP **409**.

### 3.4 UI Changes

**Back-Office**
- Add a "Subscription Plan" settings page showing the current price, an edit form, and the price history.
- Show a confirmation before saving: *"New subscribers will pay Rp [new] immediately. Existing subscribers will pay Rp [new] from their next renewal."*
- Validation: price is required, is an integer, and is > 0.

**Customer App**
- The paywall always shows the price from `GET /subscription-plan` (never hardcoded).
- For subscribers where `price_change_pending = true`, show an account notice: *"Your subscription price will change from Rp [price_paid] to Rp [next_renewal_price] on your next renewal ([expires_at date])."*
- On a `PRICE_CHANGED` error, refresh the displayed price and ask the user to confirm again.

### 3.5 Edge Cases

| # | Scenario | Expected behavior | HTTP / Code |
|---|---|---|---|
| E-CR1 | Admin changes the price while a user is on the paywall | Payment rejected; UI shows the new price | 409 `PRICE_CHANGED` |
| E-CR2 | Admin sets the price to 0 or a negative number | Rejected | 400 `INVALID_PRICE` |
| E-CR3 | Non-admin calls `PUT /admin/subscription-plan` | Rejected | 403 `FORBIDDEN` |
| E-CR4 | Subscriber renews after 2 price changes | Charged the latest price | — |
| E-CR5 | Free user calls the renew endpoint (no subscription) | Rejected; must subscribe instead | 400 `NO_ACTIVE_SUBSCRIPTION` |
| E-CR6 | Admin "changes" the price to the same value | No-op; no history row created | 200 |

### 3.6 Acceptance Criteria
- [ ] Given a subscriber who paid Rp 49,000, when the admin changes the price to Rp 59,000, then the subscriber's `price_paid` remains Rp 49,000 until `expires_at`.
- [ ] Given that subscriber, when they renew, then they are charged Rp 59,000 and `price_paid` becomes Rp 59,000.
- [ ] Given a new user after the price change, when they subscribe, then they are charged Rp 59,000.
- [ ] Given a pending price change, when the subscriber views their account, then they see the notice with the old price, new price, and renewal date.
- [ ] Given a user on the paywall, when the admin changes the price before they pay, then payment returns 409 `PRICE_CHANGED` and the UI shows the new price.
- [ ] Given an admin, when they update the price, then a row is written to the price history.
- [ ] One-time book purchase prices are unaffected by any subscription price change.

---

## 4. Test Cases (Backend)

| ID | Test | Expected |
|---|---|---|
| T1 | `/me` for active subscriber | `plan_status = subscribed` |
| T2 | `/me` for free user with purchases | `plan_status = free` |
| T3 | `/me` for expired subscriber | `plan_status = free` |
| T4 | Price change → existing subscriber's `price_paid` | Unchanged |
| T5 | Price change → renew | Charged new price; `expires_at` extended |
| T6 | Price change → new subscriber | Charged new price |
| T7 | Payment with stale `expected_price` | 409 `PRICE_CHANGED` |
| T8 | Admin sets price ≤ 0 | 400 `INVALID_PRICE` |
| T9 | Non-admin updates price | 403 |
| T10 | Renew without subscription | 400 `NO_ACTIVE_SUBSCRIPTION` |

---

## 5. Out of Scope
- Automatic renewal via a scheduler or cron (renewal is triggered manually/mocked for the MVP)
- Proration or refunds on price decreases
- Multiple plans (annual, family). The data model supports them, but the UI does not.
- Notifying subscribers by email about price changes (in-app notice only)

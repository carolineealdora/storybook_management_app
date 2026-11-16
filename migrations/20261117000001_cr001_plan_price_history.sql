-- CR-001-2: plan identity, admin audit on price changes, price history.
ALTER TABLE subscription_plans
    ADD COLUMN code       VARCHAR(50)  NOT NULL DEFAULT 'monthly',
    ADD COLUMN name       VARCHAR(200) NOT NULL DEFAULT 'Monthly Subscription',
    ADD COLUMN updated_by UUID REFERENCES admins(id); -- NULL for the seeded row
CREATE UNIQUE INDEX uq_subscription_plans_code ON subscription_plans(code);

CREATE TABLE subscription_plan_price_history (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    plan_id    UUID NOT NULL REFERENCES subscription_plans(id),
    old_price  INTEGER NOT NULL CHECK (old_price > 0),
    new_price  INTEGER NOT NULL CHECK (new_price > 0 AND new_price <> old_price), -- E-CR6: same price is never a change
    changed_by UUID NOT NULL REFERENCES admins(id),
    changed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_plan_price_history_plan ON subscription_plan_price_history(plan_id, changed_at DESC);

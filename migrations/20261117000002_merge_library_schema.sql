-- STORY-3 / ADR-010: this repo owns the schema shared with storybook_library_app.
-- Merges what that app's own migrations added and this schema lacked.

-- Forgot PIN? lockout, separate from the PIN lockout (library app, PRD E13).
ALTER TABLE users
    ADD COLUMN password_failed_attempts INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN password_locked_until    TIMESTAMPTZ;

-- The library app logs in with lower(email) = lower(?). Fails if case-duplicates already exist.
CREATE UNIQUE INDEX uq_credentials_email_lower ON credentials (lower(email));

-- Foreign-key indexes.
CREATE INDEX idx_subscription_orders_plan_id ON subscription_orders(plan_id);
CREATE INDEX idx_subscription_plans_updated_by ON subscription_plans(updated_by);
CREATE INDEX idx_plan_price_history_changed_by ON subscription_plan_price_history(changed_by);

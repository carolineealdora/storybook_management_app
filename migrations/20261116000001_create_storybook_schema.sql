-- Storybook library schema (PRD §12.2). Money = INTEGER IDR. Enums = VARCHAR + CHECK.

CREATE TABLE users (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name                VARCHAR(200) NOT NULL,
    pin_hash            VARCHAR(255),
    pin_failed_attempts INTEGER NOT NULL DEFAULT 0,
    pin_locked_until    TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE admins (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name       VARCHAR(200) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE credentials (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email         VARCHAR(255) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    account_type  VARCHAR(10) NOT NULL CHECK (account_type IN ('parent', 'admin')),
    user_id       UUID UNIQUE REFERENCES users(id),
    admin_id      UUID UNIQUE REFERENCES admins(id),
    last_login_at TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (
        (account_type = 'parent' AND user_id IS NOT NULL AND admin_id IS NULL) OR
        (account_type = 'admin' AND admin_id IS NOT NULL AND user_id IS NULL)
    )
);

CREATE TABLE refresh_tokens (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    credential_id UUID NOT NULL REFERENCES credentials(id),
    token_hash    VARCHAR(64) NOT NULL UNIQUE, -- SHA-256 hex of the opaque token
    expires_at    TIMESTAMPTZ NOT NULL,
    revoked_at    TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_refresh_tokens_credential_id ON refresh_tokens(credential_id);

CREATE TABLE books (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title           VARCHAR(255) NOT NULL,
    author          VARCHAR(255) NOT NULL,
    description     TEXT NOT NULL,
    age_type        VARCHAR(10) NOT NULL CHECK (age_type IN ('range', 'plus')),
    age_min         INTEGER NOT NULL CHECK (age_min >= 0),
    age_max         INTEGER,
    price           INTEGER NOT NULL CHECK (price >= 0), -- IDR, 0 = free
    status          VARCHAR(20) NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published')),
    epub_file_name  VARCHAR(255) NOT NULL,
    epub_size_bytes INTEGER NOT NULL,
    epub_content    BYTEA NOT NULL, -- never selected in list queries
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at      TIMESTAMPTZ,
    CHECK (
        (age_type = 'range' AND age_max IS NOT NULL AND age_min < age_max) OR
        (age_type = 'plus' AND age_max IS NULL)
    )
);
CREATE INDEX idx_books_showcase ON books(created_at DESC) WHERE status = 'published' AND deleted_at IS NULL;

CREATE TABLE book_files ( -- cover image only, one per book
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    book_id    UUID NOT NULL UNIQUE REFERENCES books(id),
    mime_type  VARCHAR(20) NOT NULL CHECK (mime_type IN ('image/jpeg', 'image/png')),
    file_name  VARCHAR(255) NOT NULL,
    size_bytes INTEGER NOT NULL,
    content    BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE subscription_plans ( -- single row, edited by admin
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    price      INTEGER NOT NULL CHECK (price > 0),
    benefits   TEXT NOT NULL,
    duration   VARCHAR(20) NOT NULL DEFAULT '1 month',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE subscriptions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id),
    plan_id         UUID NOT NULL REFERENCES subscription_plans(id),
    renewed_from_id UUID REFERENCES subscriptions(id),
    price_paid      INTEGER NOT NULL CHECK (price_paid >= 0),
    status          VARCHAR(20) NOT NULL CHECK (status IN ('active', 'cancelled', 'expired')),
    started_at      TIMESTAMPTZ NOT NULL,
    expires_at      TIMESTAMPTZ NOT NULL,
    cancelled_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_subscriptions_user_id ON subscriptions(user_id, started_at DESC);
CREATE UNIQUE INDEX uq_subscriptions_one_active ON subscriptions(user_id) WHERE status = 'active';
CREATE INDEX idx_subscriptions_due ON subscriptions(expires_at) WHERE status = 'active';

CREATE TABLE book_orders (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id),
    book_id         UUID NOT NULL REFERENCES books(id),
    price           INTEGER NOT NULL CHECK (price >= 0), -- price shown at checkout
    status          VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'paid', 'failed')),
    idempotency_key VARCHAR(100) NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    paid_at         TIMESTAMPTZ,
    UNIQUE (user_id, idempotency_key)
);
CREATE INDEX idx_book_orders_user_id ON book_orders(user_id, created_at DESC);
CREATE INDEX idx_book_orders_book_id ON book_orders(book_id);

CREATE TABLE subscription_orders (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id),
    plan_id         UUID NOT NULL REFERENCES subscription_plans(id),
    subscription_id UUID REFERENCES subscriptions(id), -- set when paid
    order_type      VARCHAR(10) NOT NULL CHECK (order_type IN ('new', 'renewal')),
    price           INTEGER NOT NULL CHECK (price >= 0),
    status          VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'paid', 'failed')),
    idempotency_key VARCHAR(100) NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    paid_at         TIMESTAMPTZ,
    UNIQUE (user_id, idempotency_key)
);
CREATE INDEX idx_subscription_orders_user_id ON subscription_orders(user_id, created_at DESC);
CREATE INDEX idx_subscription_orders_subscription_id ON subscription_orders(subscription_id);

CREATE TABLE purchases (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       UUID NOT NULL REFERENCES users(id),
    book_id       UUID NOT NULL REFERENCES books(id),
    book_order_id UUID NOT NULL UNIQUE REFERENCES book_orders(id),
    price_paid    INTEGER NOT NULL CHECK (price_paid >= 0),
    purchased_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, book_id)
);
CREATE INDEX idx_purchases_book_id ON purchases(book_id);

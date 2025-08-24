CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS btree_gist;
CREATE EXTENSION IF NOT EXISTS citext;

CREATE TABLE companies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email CITEXT NOT NULL,
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('viewer', 'operator', 'admin')),
    company_id UUID REFERENCES companies(id),
    last_login TIMESTAMPTZ,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE resources (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    lead_time_min INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE coupons (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code CITEXT NOT NULL UNIQUE,
    amount_off_cents INTEGER,
    percent_off NUMERIC(5,2),
    valid_from TIMESTAMPTZ,
    valid_to TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (
        (amount_off_cents IS NOT null AND percent_off IS null) OR
        (amount_off_cents IS null AND percent_off IS NOT null)
    ),
    CHECK (percent_off IS null OR percent_off BETWEEN 0 AND 100),
    CHECK (amount_off_cents IS null OR amount_off_cents >= 0)
);

CREATE TABLE reservations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    resource_id UUID NOT NULL REFERENCES resources(id),
    user_id UUID NOT NULL REFERENCES users(id),
    slot TSTZRANGE NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('confirmed', 'canceled')) DEFAULT 'confirmed',
    price_cents INTEGER NOT NULL DEFAULT 0,
    coupon_id UUID REFERENCES coupons(id),
    note TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE reservations
ADD CONSTRAINT reservations_no_overlap
EXCLUDE USING gist (resource_id WITH =, slot WITH &&);

CREATE TABLE notification_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    kind TEXT NOT NULL CHECK (kind IN ('email', 'webhook')),
    topic TEXT NOT NULL DEFAULT 'reservation_created',
    payload JSONB NOT NULL,
    run_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    attempts INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL CHECK (status IN ('queued', 'done', 'error')) DEFAULT 'queued',
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE idempotency_keys (
    key UUID NOT NULL,
    user_id UUID NOT NULL REFERENCES users(id),
    endpoint TEXT NOT NULL,
    request_hash TEXT NOT NULL,
    response_body_hash TEXT,
    status TEXT NOT NULL CHECK (status IN ('processing', 'completed', 'failed')) DEFAULT 'processing',
    result_reservation_id UUID REFERENCES reservations(id),
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (key, user_id)
);

CREATE UNIQUE INDEX idx_users_email_active_unique ON users(email) WHERE is_active = true;

CREATE INDEX idx_users_email ON users(email);

CREATE INDEX idx_users_company_id_active ON users(company_id) WHERE is_active = true;
CREATE INDEX idx_reservations_resource_id ON reservations(resource_id);
CREATE INDEX idx_reservations_user_id ON reservations(user_id);
CREATE INDEX idx_reservations_slot ON reservations USING gist(slot);
CREATE INDEX idx_notification_jobs_status_run_at ON notification_jobs(status, run_at);
CREATE INDEX idx_idempotency_keys_expires_at ON idempotency_keys(expires_at);
CREATE INDEX idx_reservations_user_created_desc ON reservations (user_id, created_at DESC, id DESC);

-- Insert initial data
INSERT INTO companies (id, name) VALUES 
    (gen_random_uuid(), 'Default Company'),
    (gen_random_uuid(), 'Test Company');

-- Insert test user (password: 'password123!')
INSERT INTO users (email, password_hash, role, company_id) 
SELECT 
    'test@example.com' AS email,
    '$2a$12$LQv3c1yqBWVHxkd0LHAkCOYz6TtxMQJqhN8/LewdBPj2hCBQWqRU6' AS password_hash, -- bcrypt hash of password123
    'admin' AS "role",
    companies.id AS company_id
FROM companies 
WHERE companies.name = 'Default Company'
LIMIT 1;

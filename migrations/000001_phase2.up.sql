CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE provider_accounts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_slug text NOT NULL UNIQUE CHECK (provider_slug ~ '^[a-z0-9][a-z0-9_-]*$'),
    display_name text NOT NULL CHECK (length(trim(display_name)) > 0),
    enabled boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE credentials (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_account_id uuid NOT NULL REFERENCES provider_accounts(id) ON DELETE CASCADE,
    ciphertext bytea NOT NULL CHECK (octet_length(ciphertext) > 16),
    nonce bytea NOT NULL CHECK (octet_length(nonce) = 12),
    key_version integer NOT NULL CHECK (key_version > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (provider_account_id)
);

CREATE TABLE balance_snapshots (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    provider_account_id uuid NOT NULL REFERENCES provider_accounts(id) ON DELETE CASCADE,
    amount numeric(36,12) NOT NULL CHECK (amount >= 0),
    currency varchar(12) NOT NULL CHECK (currency ~ '^[A-Z0-9]{2,12}$'),
    collected_at timestamptz NOT NULL,
    provider_timestamp timestamptz,
    source_hash text NOT NULL CHECK (length(source_hash) = 64),
    UNIQUE (provider_account_id, source_hash)
);
CREATE INDEX balance_snapshots_account_collected_idx ON balance_snapshots(provider_account_id, collected_at DESC);

CREATE TABLE collector_runs (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    provider_account_id uuid NOT NULL REFERENCES provider_accounts(id) ON DELETE CASCADE,
    idempotency_key text NOT NULL,
    started_at timestamptz NOT NULL,
    finished_at timestamptz,
    state text NOT NULL CHECK (state IN ('running','ok','error')),
    error_kind text,
    error_message text,
    UNIQUE (provider_account_id, idempotency_key)
);
CREATE INDEX collector_runs_account_started_idx ON collector_runs(provider_account_id, started_at DESC);

CREATE TABLE users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    username text NOT NULL UNIQUE CHECK (username = lower(username)),
    password_hash text NOT NULL,
    is_active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE admin_sessions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash bytea NOT NULL UNIQUE CHECK (octet_length(token_hash) = 32),
    csrf_hash bytea NOT NULL CHECK (octet_length(csrf_hash) = 32),
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,
    last_seen_at timestamptz NOT NULL DEFAULT now(),
    revoked_at timestamptz,
    CHECK (expires_at > created_at)
);
CREATE INDEX admin_sessions_active_idx ON admin_sessions(token_hash) WHERE revoked_at IS NULL;
CREATE INDEX admin_sessions_expiry_idx ON admin_sessions(expires_at);

CREATE TABLE audit_logs (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    created_at timestamptz NOT NULL DEFAULT now(),
    actor_user_id uuid REFERENCES users(id) ON DELETE SET NULL,
    action text NOT NULL,
    target_type text NOT NULL,
    target_id text NOT NULL,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(metadata) = 'object')
);
CREATE INDEX audit_logs_created_idx ON audit_logs(created_at DESC);
CREATE INDEX audit_logs_actor_idx ON audit_logs(actor_user_id, created_at DESC);

-- Pendapatan terverifikasi sengaja tidak diturunkan dari balance_snapshots.
CREATE TABLE verified_earnings (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    provider_account_id uuid NOT NULL REFERENCES provider_accounts(id) ON DELETE CASCADE,
    amount numeric(36,12) NOT NULL CHECK (amount >= 0),
    currency varchar(12) NOT NULL CHECK (currency ~ '^[A-Z0-9]{2,12}$'),
    period_start timestamptz NOT NULL,
    period_end timestamptz NOT NULL,
    evidence_ref text NOT NULL,
    verified_at timestamptz NOT NULL,
    verified_by uuid REFERENCES users(id) ON DELETE SET NULL,
    CHECK (period_end > period_start),
    UNIQUE (provider_account_id, period_start, period_end, evidence_ref)
);

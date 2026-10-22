-- DESIGN ONLY: not applied; requires review before production.
CREATE TABLE provider_accounts (id uuid PRIMARY KEY, provider_slug text NOT NULL, display_name text NOT NULL, created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE credentials (id uuid PRIMARY KEY, provider_account_id uuid NOT NULL REFERENCES provider_accounts(id), ciphertext bytea NOT NULL, nonce bytea NOT NULL, key_version integer NOT NULL);
CREATE TABLE balance_snapshots (id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY, provider_account_id uuid NOT NULL REFERENCES provider_accounts(id), amount numeric(36,12) NOT NULL, currency text NOT NULL, collected_at timestamptz NOT NULL, provider_timestamp timestamptz, source_hash text);
CREATE TABLE collector_runs (id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY, provider_account_id uuid NOT NULL REFERENCES provider_accounts(id), started_at timestamptz NOT NULL, finished_at timestamptz, state text NOT NULL, error_kind text);
CREATE TABLE audit_logs (id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY, created_at timestamptz NOT NULL DEFAULT now(), actor_id text NOT NULL, action text NOT NULL, target_id text NOT NULL, metadata jsonb NOT NULL DEFAULT '{}'::jsonb);

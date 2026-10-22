ALTER TABLE provider_accounts ADD COLUMN account_key text NOT NULL DEFAULT 'default';
ALTER TABLE provider_accounts DROP CONSTRAINT provider_accounts_provider_slug_key;
ALTER TABLE provider_accounts ADD CONSTRAINT provider_accounts_slug_account_key_unique UNIQUE(provider_slug, account_key);
ALTER TABLE provider_accounts ADD CONSTRAINT provider_accounts_account_key_valid CHECK (account_key ~ '^[a-z0-9][a-z0-9_-]{0,62}$');
CREATE INDEX provider_accounts_enabled_idx ON provider_accounts(enabled, provider_slug);

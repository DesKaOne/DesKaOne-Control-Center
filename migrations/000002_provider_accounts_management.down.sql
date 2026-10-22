-- Reversible only when at most one account exists per provider.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM provider_accounts GROUP BY provider_slug HAVING count(*) > 1) THEN
        RAISE EXCEPTION 'cannot restore provider_slug uniqueness while multiple accounts exist';
    END IF;
END $$;
DROP INDEX IF EXISTS provider_accounts_enabled_idx;
ALTER TABLE provider_accounts DROP CONSTRAINT provider_accounts_account_key_valid;
ALTER TABLE provider_accounts DROP CONSTRAINT provider_accounts_slug_account_key_unique;
ALTER TABLE provider_accounts DROP COLUMN account_key;
ALTER TABLE provider_accounts ADD CONSTRAINT provider_accounts_provider_slug_key UNIQUE(provider_slug);

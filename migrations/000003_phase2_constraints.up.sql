-- NOT VALID avoids scanning/rejecting historical rows while enforcing new writes.
ALTER TABLE collector_runs ADD CONSTRAINT collector_runs_idempotency_key_nonempty CHECK (length(trim(idempotency_key)) > 0) NOT VALID;
ALTER TABLE balance_snapshots ADD CONSTRAINT balance_snapshots_source_hash_hex CHECK (source_hash ~ '^[0-9a-f]{64}$') NOT VALID;

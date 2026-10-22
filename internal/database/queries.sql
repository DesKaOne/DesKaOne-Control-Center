-- name: EnsureProviderAccount :one
INSERT INTO provider_accounts (provider_slug, display_name)
VALUES ($1, $2)
ON CONFLICT (provider_slug) DO UPDATE SET display_name = EXCLUDED.display_name, updated_at = now()
RETURNING id;

-- name: LatestBalances :many
SELECT DISTINCT ON (pa.id) pa.provider_slug, bs.amount::text AS amount, bs.currency,
       bs.collected_at, cr.state, COALESCE(cr.error_message, '') AS error_message
FROM provider_accounts pa
LEFT JOIN balance_snapshots bs ON bs.provider_account_id = pa.id
LEFT JOIN LATERAL (
    SELECT state, error_message FROM collector_runs
    WHERE provider_account_id = pa.id ORDER BY started_at DESC LIMIT 1
) cr ON true
ORDER BY pa.id, bs.collected_at DESC NULLS LAST;

-- name: FindUserByUsername :one
SELECT id, username, password_hash, is_active FROM users WHERE username = $1;

-- name: CreateSession :exec
INSERT INTO admin_sessions (user_id, token_hash, csrf_hash, expires_at) VALUES ($1, $2, $3, $4);

-- name: FindSession :one
SELECT s.user_id, u.username, s.csrf_hash, s.expires_at
FROM admin_sessions s JOIN users u ON u.id = s.user_id
WHERE s.token_hash = $1 AND s.revoked_at IS NULL AND s.expires_at > now() AND u.is_active;

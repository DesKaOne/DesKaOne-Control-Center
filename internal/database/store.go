package database

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/DesKaOne/DesKaOne-Control-Center/internal/auth"
	"github.com/DesKaOne/DesKaOne-Control-Center/internal/collectors"
	dbsqlc "github.com/DesKaOne/DesKaOne-Control-Center/internal/database/sqlc"
	"github.com/DesKaOne/DesKaOne-Control-Center/internal/vault"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool    *pgxpool.Pool
	queries *dbsqlc.Queries
	timeout time.Duration
}

func Open(ctx context.Context, databaseURL string, timeout time.Duration) (*Store, error) {
	if databaseURL == "" {
		return nil, errors.New("DKO_DATABASE_URL belum diatur")
	}
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("konfigurasi database tidak valid: %w", err)
	}
	config.MaxConns = 10
	config.MinConns = 1
	config.MaxConnLifetime = 30 * time.Minute
	config.MaxConnIdleTime = 5 * time.Minute
	openCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	pool, err := pgxpool.NewWithConfig(openCtx, config)
	if err != nil {
		return nil, fmt.Errorf("membuka pool database: %w", err)
	}
	if err := pool.Ping(openCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("database tidak tersedia: %w", err)
	}
	return &Store{pool: pool, queries: dbsqlc.New(pool), timeout: timeout}, nil
}
func (s *Store) Close() { s.pool.Close() }
func (s *Store) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, s.timeout)
}

func (s *Store) EnsureProviders(ctx context.Context, providers []collectors.Collector) error {
	ctx, cancel := s.withTimeout(ctx)
	defer cancel()
	for _, p := range providers {
		if _, err := s.queries.EnsureProviderAccount(ctx, dbsqlc.EnsureProviderAccountParams{ProviderSlug: p.ID(), DisplayName: p.ID()}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) RecordReading(ctx context.Context, r collectors.Reading, idempotencyKey string) error {
	ctx, cancel := s.withTimeout(ctx)
	defer cancel()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var accountID string
	if err := tx.QueryRow(ctx, `SELECT id::text FROM provider_accounts WHERE provider_slug=$1`, r.Provider).Scan(&accountID); err != nil {
		return err
	}
	started := r.CheckedAt
	if started.IsZero() {
		started = time.Now().UTC()
	}
	var runID int64
	err = tx.QueryRow(ctx, `INSERT INTO collector_runs(provider_account_id,idempotency_key,started_at,finished_at,state,error_kind,error_message)
		VALUES($1,$2,$3,$3,$4,$5,$6) ON CONFLICT(provider_account_id,idempotency_key) DO UPDATE SET idempotency_key=EXCLUDED.idempotency_key RETURNING id`,
		accountID, idempotencyKey, started, r.State, errorKind(r), nullable(r.Message)).Scan(&runID)
	if err != nil {
		return err
	}
	if r.State == "ok" {
		source := sha256.Sum256([]byte(r.Provider + "\x00" + r.Balance + "\x00" + r.Currency + "\x00" + started.UTC().Format(time.RFC3339Nano)))
		if _, err = tx.Exec(ctx, `INSERT INTO balance_snapshots(provider_account_id,amount,currency,collected_at,source_hash)
			VALUES($1,$2::numeric,$3,$4,$5) ON CONFLICT(provider_account_id,source_hash) DO NOTHING`, accountID, r.Balance, r.Currency, started, hex.EncodeToString(source[:])); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
func errorKind(r collectors.Reading) any {
	if r.State == "error" {
		return "provider_error"
	}
	return nil
}
func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func (s *Store) LatestReadings(ctx context.Context) (map[string]collectors.Reading, error) {
	ctx, cancel := s.withTimeout(ctx)
	defer cancel()
	rows, err := s.pool.Query(ctx, `SELECT pa.provider_slug, COALESCE(bs.amount::text,''), COALESCE(bs.currency,''),
		COALESCE(bs.collected_at,'0001-01-01'::timestamptz), COALESCE(cr.state,'belum diperiksa'), COALESCE(cr.error_message,'')
		FROM provider_accounts pa
		LEFT JOIN LATERAL (SELECT amount,currency,collected_at FROM balance_snapshots WHERE provider_account_id=pa.id ORDER BY collected_at DESC,id DESC LIMIT 1) bs ON true
		LEFT JOIN LATERAL (SELECT state,error_message FROM collector_runs WHERE provider_account_id=pa.id ORDER BY started_at DESC,id DESC LIMIT 1) cr ON true`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]collectors.Reading{}
	for rows.Next() {
		var r collectors.Reading
		if err := rows.Scan(&r.Provider, &r.Balance, &r.Currency, &r.CheckedAt, &r.State, &r.Message); err != nil {
			return nil, err
		}
		out[r.Provider] = r
	}
	return out, rows.Err()
}

func (s *Store) FindUser(ctx context.Context, username string) (id, passwordHash string, active bool, err error) {
	ctx, cancel := s.withTimeout(ctx)
	defer cancel()
	err = s.pool.QueryRow(ctx, `SELECT id::text,password_hash,is_active FROM users WHERE username=lower($1)`, username).Scan(&id, &passwordHash, &active)
	return
}
func (s *Store) CreateSession(ctx context.Context, userID string, tokenHash, csrfHash []byte, expires time.Time) error {
	ctx, cancel := s.withTimeout(ctx)
	defer cancel()
	_, err := s.pool.Exec(ctx, `INSERT INTO admin_sessions(user_id,token_hash,csrf_hash,expires_at) VALUES($1,$2,$3,$4)`, userID, tokenHash, csrfHash, expires)
	return err
}
func (s *Store) Session(ctx context.Context, tokenHash []byte) (auth.Session, error) {
	ctx, cancel := s.withTimeout(ctx)
	defer cancel()
	var x auth.Session
	err := s.pool.QueryRow(ctx, `SELECT s.user_id::text,u.username,s.csrf_hash,s.expires_at FROM admin_sessions s JOIN users u ON u.id=s.user_id WHERE s.token_hash=$1 AND s.revoked_at IS NULL AND s.expires_at>now() AND u.is_active`, tokenHash).Scan(&x.UserID, &x.Username, &x.CSRFHash, &x.ExpiresAt)
	return x, err
}
func (s *Store) RevokeSession(ctx context.Context, tokenHash []byte) error {
	ctx, cancel := s.withTimeout(ctx)
	defer cancel()
	_, err := s.pool.Exec(ctx, `UPDATE admin_sessions SET revoked_at=now() WHERE token_hash=$1`, tokenHash)
	return err
}
func (s *Store) UpsertBootstrapAdmin(ctx context.Context, username, passwordHash string) error {
	ctx, cancel := s.withTimeout(ctx)
	defer cancel()
	_, err := s.pool.Exec(ctx, `INSERT INTO users(username,password_hash) VALUES(lower($1),$2) ON CONFLICT(username) DO NOTHING`, username, passwordHash)
	return err
}

func (s *Store) PutCredential(ctx context.Context, providerSlug string, secret vault.Ciphertext) error {
	ctx, cancel := s.withTimeout(ctx)
	defer cancel()
	tag, err := s.pool.Exec(ctx, `INSERT INTO credentials(provider_account_id,ciphertext,nonce,key_version)
		SELECT id,$2,$3,$4 FROM provider_accounts WHERE provider_slug=$1
		ON CONFLICT(provider_account_id) DO UPDATE SET ciphertext=EXCLUDED.ciphertext,nonce=EXCLUDED.nonce,key_version=EXCLUDED.key_version,updated_at=now()`, providerSlug, secret.Data, secret.Nonce, secret.Version)
	if err == nil && tag.RowsAffected() == 0 {
		return errors.New("provider account tidak ditemukan")
	}
	return err
}

func (s *Store) Credential(ctx context.Context, providerSlug string) (vault.Ciphertext, error) {
	ctx, cancel := s.withTimeout(ctx)
	defer cancel()
	var out vault.Ciphertext
	err := s.pool.QueryRow(ctx, `SELECT key_version,nonce,ciphertext FROM credentials c JOIN provider_accounts p ON p.id=c.provider_account_id WHERE p.provider_slug=$1`, providerSlug).Scan(&out.Version, &out.Nonce, &out.Data)
	return out, err
}

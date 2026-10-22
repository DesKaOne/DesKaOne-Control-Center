//go:build integration

package database

import (
	"bytes"
	"context"
	"fmt"
	"github.com/DesKaOne/DesKaOne-Control-Center/internal/collectors"
	"github.com/DesKaOne/DesKaOne-Control-Center/internal/credentials"
	"github.com/DesKaOne/DesKaOne-Control-Center/internal/vault"
	"os"
	"sync"
	"testing"
	"time"
)

type stubCollector struct{ id string }

func (s stubCollector) ID() string                               { return s.id }
func (stubCollector) Enabled() bool                              { return true }
func (stubCollector) Collect(context.Context) collectors.Reading { return collectors.Reading{} }

func TestPostgresRecordIsIdempotentAndRetainsBalanceOnFailure(t *testing.T) {
	url := os.Getenv("DKO_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("atur DKO_TEST_DATABASE_URL untuk integration test")
	}
	if err := ValidateTestDatabaseURL(url); err != nil {
		t.Fatalf("target integration test ditolak: %v", err)
	}
	s, err := Open(context.Background(), url, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	id := fmt.Sprintf("integration-%d", time.Now().UnixNano())
	if err = s.EnsureProviders(context.Background(), []collectors.Collector{stubCollector{id}}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	ok := collectors.Reading{Provider: id, Balance: "12.50", Currency: "USD", State: "ok", CheckedAt: now}
	if err = s.RecordReading(context.Background(), ok, "same-key"); err != nil {
		t.Fatal(err)
	}
	if err = s.RecordReading(context.Background(), ok, "same-key"); err != nil {
		t.Fatal(err)
	}
	fail := collectors.Reading{Provider: id, State: "error", Message: "fixture gagal", CheckedAt: now.Add(time.Second)}
	if err = s.RecordReading(context.Background(), fail, "failure-key"); err != nil {
		t.Fatal(err)
	}
	got, err := s.LatestReadings(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got[id].Balance != "12.500000000000" || got[id].State != "error" {
		t.Fatalf("saldo gagal dipertahankan: %+v", got[id])
	}
	var accountID string
	if err = s.pool.QueryRow(context.Background(), `SELECT id::text FROM provider_accounts WHERE provider_slug=$1 AND account_key='default'`, id).Scan(&accountID); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			reading := collectors.Reading{Provider: id, Balance: fmt.Sprintf("%d.00", n+20), Currency: "USD", State: "ok", CheckedAt: now.Add(time.Duration(n+2) * time.Second)}
			errs <- s.RecordReading(context.Background(), reading, "concurrent-key")
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	var runs, snapshots int
	if err = s.pool.QueryRow(context.Background(), `SELECT count(*) FROM collector_runs WHERE provider_account_id=$1 AND idempotency_key='concurrent-key'`, accountID).Scan(&runs); err != nil {
		t.Fatal(err)
	}
	if err = s.pool.QueryRow(context.Background(), `SELECT count(*) FROM balance_snapshots WHERE provider_account_id=$1 AND collected_at>$2`, accountID, now).Scan(&snapshots); err != nil {
		t.Fatal(err)
	}
	if runs != 1 || snapshots != 1 {
		t.Fatalf("idempotency concurrency gagal: runs=%d snapshots=%d", runs, snapshots)
	}
}

func TestPostgresCredentialEncryptionIsolationAndRollback(t *testing.T) {
	url := os.Getenv("DKO_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("atur DKO_TEST_DATABASE_URL untuk integration test")
	}
	if err := ValidateTestDatabaseURL(url); err != nil {
		t.Fatalf("target integration test ditolak: %v", err)
	}
	s, err := Open(context.Background(), url, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	suffix := fmt.Sprint(time.Now().UnixNano())
	a, err := s.CreateProviderAccount(ctx, "proxyrack", "a"+suffix, "A")
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.CreateProviderAccount(ctx, "proxyrack", "b"+suffix, "B")
	if err != nil {
		t.Fatal(err)
	}
	svc, err := credentials.New(s, bytes.Repeat([]byte{9}, 32))
	if err != nil {
		t.Fatal(err)
	}
	if err = svc.Put(ctx, a, map[string]string{"api_key": "secret-a"}); err != nil {
		t.Fatal(err)
	}
	if err = svc.Put(ctx, b, map[string]string{"api_key": "secret-b"}); err != nil {
		t.Fatal(err)
	}
	ga, err := svc.Get(ctx, a)
	if err != nil || ga["api_key"] != "secret-a" {
		t.Fatal("credential akun A gagal")
	}
	gb, err := svc.Get(ctx, b)
	if err != nil || gb["api_key"] != "secret-b" {
		t.Fatal("credential akun B tercampur")
	}
	var plaintext bool
	if err = s.pool.QueryRow(ctx, `SELECT position(convert_to('secret-a','UTF8') in ciphertext)>0 FROM credentials WHERE provider_account_id=$1`, a).Scan(&plaintext); err != nil {
		t.Fatal(err)
	}
	if plaintext {
		t.Fatal("plaintext ditemukan dalam ciphertext")
	}
	encryptedB, err := s.Credential(ctx, b)
	if err != nil {
		t.Fatal(err)
	}
	rawVault, err := vault.New(bytes.Repeat([]byte{9}, 32))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = rawVault.Decrypt(encryptedB, []byte(a)); err == nil {
		t.Fatal("AAD tidak mencegah swap ciphertext antar akun")
	}
	if err = s.UpdateProviderAccount(ctx, b, "B", false); err != nil {
		t.Fatal(err)
	}
	accounts, err := s.ListProviderAccounts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, account := range accounts {
		if account.ID == b && account.Enabled {
			t.Fatal("akun gagal dinonaktifkan")
		}
	}
	bad := collectors.Reading{Provider: a, Balance: "bukan-angka", Currency: "USD", State: "ok", CheckedAt: time.Now().UTC()}
	if err = s.RecordReading(ctx, bad, "rollback-key"); err == nil {
		t.Fatal("nilai malformed harus gagal")
	}
	var count int
	if err = s.pool.QueryRow(ctx, `SELECT count(*) FROM collector_runs WHERE provider_account_id=$1 AND idempotency_key='rollback-key'`, a).Scan(&count); err != nil || count != 0 {
		t.Fatalf("transaksi tidak rollback: count=%d err=%v", count, err)
	}
}

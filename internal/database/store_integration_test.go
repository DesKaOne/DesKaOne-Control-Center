//go:build integration

package database

import (
	"bytes"
	"context"
	"fmt"
	"github.com/DesKaOne/DesKaOne-Control-Center/internal/collectors"
	"github.com/DesKaOne/DesKaOne-Control-Center/internal/credentials"
	"os"
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
}

func TestPostgresCredentialEncryptionIsolationAndRollback(t *testing.T) {
	url := os.Getenv("DKO_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("atur DKO_TEST_DATABASE_URL untuk integration test")
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
	bad := collectors.Reading{Provider: a, Balance: "bukan-angka", Currency: "USD", State: "ok", CheckedAt: time.Now().UTC()}
	if err = s.RecordReading(ctx, bad, "rollback-key"); err == nil {
		t.Fatal("nilai malformed harus gagal")
	}
	var count int
	if err = s.pool.QueryRow(ctx, `SELECT count(*) FROM collector_runs WHERE provider_account_id=$1 AND idempotency_key='rollback-key'`, a).Scan(&count); err != nil || count != 0 {
		t.Fatalf("transaksi tidak rollback: count=%d err=%v", count, err)
	}
}

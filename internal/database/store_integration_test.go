//go:build integration

package database

import (
	"context"
	"github.com/DesKaOne/DesKaOne-Control-Center/internal/collectors"
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
	id := "integration-" + time.Now().Format("150405.000000")
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

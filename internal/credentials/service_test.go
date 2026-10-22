package credentials

import (
	"bytes"
	"context"
	"github.com/DesKaOne/DesKaOne-Control-Center/internal/vault"
	"testing"
)

type memoryRepo struct{ value vault.Ciphertext }

func (m *memoryRepo) PutCredential(_ context.Context, _ string, v vault.Ciphertext) error {
	m.value = v
	return nil
}
func (m *memoryRepo) Credential(context.Context, string) (vault.Ciphertext, error) {
	return m.value, nil
}
func TestServiceRoundTrip(t *testing.T) {
	repo := &memoryRepo{}
	svc, err := New(repo, bytes.Repeat([]byte{4}, 32))
	if err != nil {
		t.Fatal(err)
	}
	if err = svc.Put(context.Background(), "provider", map[string]string{"token": "rahasia"}); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(repo.value.Data, []byte("rahasia")) {
		t.Fatal("credential tersimpan sebagai plaintext")
	}
	got, err := svc.Get(context.Background(), "provider")
	if err != nil || got["token"] != "rahasia" {
		t.Fatalf("round trip gagal: %v", err)
	}
}

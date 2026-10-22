package credentials

import (
	"bytes"
	"context"
	"github.com/DesKaOne/DesKaOne-Control-Center/internal/vault"
	"testing"
)

type memoryRepo struct {
	value  vault.Ciphertext
	values map[string]vault.Ciphertext
	fail   bool
}

func (m *memoryRepo) PutCredential(_ context.Context, id string, v vault.Ciphertext) error {
	if m.fail {
		return context.Canceled
	}
	m.value = v
	if m.values == nil {
		m.values = map[string]vault.Ciphertext{}
	}
	m.values[id] = v
	return nil
}
func (m *memoryRepo) Credential(_ context.Context, id string) (vault.Ciphertext, error) {
	if m.fail {
		return vault.Ciphertext{}, context.Canceled
	}
	if m.values != nil {
		return m.values[id], nil
	}
	return m.value, nil
}

func TestServiceDatabaseFailure(t *testing.T) {
	svc, err := New(&memoryRepo{fail: true}, bytes.Repeat([]byte{3}, 32))
	if err != nil {
		t.Fatal(err)
	}
	if err = svc.Put(context.Background(), "a", map[string]string{"token": "x"}); err == nil {
		t.Fatal("kegagalan database harus diteruskan")
	}
	if _, err = svc.Get(context.Background(), "a"); err == nil {
		t.Fatal("kegagalan baca harus diteruskan")
	}
}

func TestServiceMultiAccountIsolation(t *testing.T) {
	svc, err := New(&memoryRepo{}, bytes.Repeat([]byte{5}, 32))
	if err != nil {
		t.Fatal(err)
	}
	_ = svc.Put(context.Background(), "a", map[string]string{"token": "A"})
	_ = svc.Put(context.Background(), "b", map[string]string{"token": "B"})
	a, err := svc.Get(context.Background(), "a")
	if err != nil || a["token"] != "A" {
		t.Fatal("akun A tercampur")
	}
	b, err := svc.Get(context.Background(), "b")
	if err != nil || b["token"] != "B" {
		t.Fatal("akun B tercampur")
	}
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

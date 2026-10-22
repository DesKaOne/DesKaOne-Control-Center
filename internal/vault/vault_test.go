package vault

import (
	"bytes"
	"testing"
)

func TestRoundTripAndRandomNonce(t *testing.T) {
	v, err := New(bytes.Repeat([]byte{7}, 32))
	if err != nil {
		t.Fatal(err)
	}
	a, err := v.Encrypt([]byte("rahasia"), []byte("provider-1"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := v.Encrypt([]byte("rahasia"), []byte("provider-1"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(a.Nonce, b.Nonce) || bytes.Equal(a.Data, b.Data) {
		t.Fatal("enkripsi harus memakai nonce acak")
	}
	got, err := v.Decrypt(a, []byte("provider-1"))
	if err != nil || string(got) != "rahasia" {
		t.Fatalf("round trip gagal: %v", err)
	}
	if _, err := v.Decrypt(a, []byte("provider-lain")); err == nil {
		t.Fatal("AAD yang salah harus ditolak")
	}
}

func TestRejectInvalidKeyAndMalformedCiphertext(t *testing.T) {
	if _, err := New([]byte("pendek")); err == nil {
		t.Fatal("key pendek harus ditolak")
	}
	v, _ := New(bytes.Repeat([]byte{1}, 32))
	if _, err := v.Decrypt(Ciphertext{Version: 99}, nil); err == nil {
		t.Fatal("versi asing harus ditolak")
	}
}

package auth

import (
	"testing"
	"time"
)

func TestPassword(t *testing.T) {
	h, err := HashPassword("password-yang-panjang")
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyPassword(h, "password-yang-panjang") || VerifyPassword(h, "salah") || VerifyPassword("malformed", "x") {
		t.Fatal("verifikasi password salah")
	}
}
func TestTokenAndCSRF(t *testing.T) {
	raw, hash, err := NewToken()
	if err != nil {
		t.Fatal(err)
	}
	if !ValidCSRF(raw, hash) || ValidCSRF("salah", hash) {
		t.Fatal("validasi token salah")
	}
}
func TestLimiter(t *testing.T) {
	l := NewLimiter(2, time.Minute)
	now := time.Now()
	if !l.Allow("ip", now) || !l.Allow("ip", now) || l.Allow("ip", now) {
		t.Fatal("limit salah")
	}
	if !l.Allow("ip", now.Add(2*time.Minute)) {
		t.Fatal("window tidak reset")
	}
}

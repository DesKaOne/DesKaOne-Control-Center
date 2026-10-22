package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/argon2"
)

const CookieName = "dko_admin_session"

type Session struct {
	UserID, Username string
	CSRFHash         []byte
	ExpiresAt        time.Time
}

func HashPassword(password string) (string, error) {
	if len(password) < 12 {
		return "", errors.New("password minimal 12 karakter")
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := argon2.IDKey([]byte(password), salt, 3, 64*1024, 2, 32)
	return fmt.Sprintf("$argon2id$v=19$m=65536,t=3,p=2$%s$%s", base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(hash)), nil
}

func VerifyPassword(encoded, password string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" {
		return false
	}
	var memory uint32
	var iterations uint32
	var parallelism uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism); err != nil {
		return false
	}
	if memory > 256*1024 || iterations > 10 || parallelism > 16 || memory < 8*1024 || iterations < 1 || parallelism < 1 {
		return false
	}
	salt, err1 := base64.RawStdEncoding.DecodeString(parts[4])
	expected, err2 := base64.RawStdEncoding.DecodeString(parts[5])
	if err1 != nil || err2 != nil || len(salt) < 8 || len(expected) != 32 {
		return false
	}
	actual := argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, uint32(len(expected)))
	return subtle.ConstantTimeCompare(actual, expected) == 1
}

func NewToken() (raw string, hash []byte, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", nil, err
	}
	raw = base64.RawURLEncoding.EncodeToString(b)
	sum := sha256.Sum256([]byte(raw))
	return raw, sum[:], nil
}
func HashToken(raw string) []byte { sum := sha256.Sum256([]byte(raw)); return sum[:] }
func ValidCSRF(raw string, expected []byte) bool {
	return subtle.ConstantTimeCompare(HashToken(raw), expected) == 1
}

type attempt struct {
	count int
	reset time.Time
}
type Limiter struct {
	mu       sync.Mutex
	attempts map[string]attempt
	limit    int
	window   time.Duration
}

func NewLimiter(limit int, window time.Duration) *Limiter {
	return &Limiter{attempts: map[string]attempt{}, limit: limit, window: window}
}
func (l *Limiter) Allow(key string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	a := l.attempts[key]
	if !now.Before(a.reset) {
		a = attempt{reset: now.Add(l.window)}
	}
	if a.count >= l.limit {
		l.attempts[key] = a
		return false
	}
	a.count++
	l.attempts[key] = a
	return true
}
func (l *Limiter) Reset(key string) { l.mu.Lock(); delete(l.attempts, key); l.mu.Unlock() }

func ParseForwardedIP(remote string) string {
	// Sengaja tidak mempercayai X-Forwarded-For tanpa konfigurasi trusted proxy.
	if i := strings.LastIndex(remote, ":"); i >= 0 {
		return strings.Trim(remote[:i], "[]")
	}
	return remote
}

func ParsePositiveInt(s string, fallback int) int {
	v, err := strconv.Atoi(s)
	if err != nil || v < 1 {
		return fallback
	}
	return v
}

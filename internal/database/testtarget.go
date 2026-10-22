package database

import (
	"errors"
	"net"
	"net/url"
	"strings"
)

// ValidateTestDatabaseURL fail-closed agar integration test tidak menyentuh DB non-test/remote.
func ValidateTestDatabaseURL(raw string) error {
	if raw == "" {
		return errors.New("DKO_TEST_DATABASE_URL belum diatur")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "postgres" && u.Scheme != "postgresql" {
		return errors.New("URL database test tidak valid")
	}
	databaseName := strings.TrimPrefix(u.Path, "/")
	if !strings.Contains(strings.ToLower(databaseName), "test") {
		return errors.New("nama database wajib mengandung kata test")
	}
	host := u.Hostname()
	if host == "" {
		return errors.New("host database test wajib eksplisit")
	}
	ip := net.ParseIP(host)
	if !strings.EqualFold(host, "localhost") && (ip == nil || !ip.IsLoopback()) {
		return errors.New("database test harus berada di host loopback")
	}
	return nil
}

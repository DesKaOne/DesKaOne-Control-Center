package config

import (
	"bufio"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Environment     string
	ListenAddr      string
	DatabaseURL     string
	MasterKey       []byte
	CookieSecure    bool
	PollInterval    time.Duration
	DatabaseTimeout time.Duration
}

func LoadDotEnv(path string) error {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		s := strings.TrimSpace(sc.Text())
		if s == "" || strings.HasPrefix(s, "#") {
			continue
		}
		k, v, ok := strings.Cut(s, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.Trim(strings.TrimSpace(v), "\"'")
		if k == "" {
			continue
		}
		if _, set := os.LookupEnv(k); !set {
			if err := os.Setenv(k, v); err != nil {
				return err
			}
		}
	}
	return sc.Err()
}
func Get(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
func PollMinutes() int {
	v, e := strconv.Atoi(Get("DKO_POLL_MINUTES", "5"))
	if e != nil || v < 1 || v > 1440 {
		return 5
	}
	return v
}

func Load() (Config, error) {
	c := Config{
		Environment:     strings.ToLower(Get("DKO_ENV", "development")),
		ListenAddr:      Get("DKO_LISTEN_ADDR", "127.0.0.1:8095"),
		DatabaseURL:     os.Getenv("DKO_DATABASE_URL"),
		PollInterval:    time.Duration(PollMinutes()) * time.Minute,
		DatabaseTimeout: 5 * time.Second,
	}
	if seconds, err := strconv.Atoi(Get("DKO_DATABASE_TIMEOUT_SECONDS", "5")); err == nil && seconds >= 1 && seconds <= 60 {
		c.DatabaseTimeout = time.Duration(seconds) * time.Second
	}
	c.CookieSecure = strings.EqualFold(Get("DKO_COOKIE_SECURE", "false"), "true")
	if raw := os.Getenv("DKO_MASTER_KEY"); raw != "" {
		key, err := base64.StdEncoding.DecodeString(raw)
		if err != nil || len(key) != 32 {
			return Config{}, errors.New("DKO_MASTER_KEY harus base64 yang mewakili tepat 32 byte")
		}
		c.MasterKey = key
	}
	if c.Environment == "production" {
		if c.DatabaseURL == "" || len(c.MasterKey) != 32 || !c.CookieSecure {
			return Config{}, errors.New("produksi mewajibkan DKO_DATABASE_URL, DKO_MASTER_KEY valid, dan DKO_COOKIE_SECURE=true")
		}
		host, _, err := net.SplitHostPort(c.ListenAddr)
		if err != nil {
			return Config{}, fmt.Errorf("DKO_LISTEN_ADDR: %w", err)
		}
		if host == "" {
			return Config{}, errors.New("DKO_LISTEN_ADDR produksi harus memiliki host eksplisit")
		}
	}
	return c, nil
}

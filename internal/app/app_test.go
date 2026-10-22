package app

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/DesKaOne/DesKaOne-Control-Center/internal/auth"
	"github.com/DesKaOne/DesKaOne-Control-Center/internal/collectors"
)

type fakeRepo struct {
	password            string
	tokenHash, csrfHash []byte
	readings            map[string]collectors.Reading
}

func (f *fakeRepo) EnsureProviders(context.Context, []collectors.Collector) error { return nil }
func (f *fakeRepo) RecordReading(_ context.Context, r collectors.Reading, _ string) error {
	if f.readings == nil {
		f.readings = map[string]collectors.Reading{}
	}
	if r.State == "ok" {
		f.readings[r.Provider] = r
	}
	return nil
}
func (f *fakeRepo) LatestReadings(context.Context) (map[string]collectors.Reading, error) {
	return f.readings, nil
}
func (f *fakeRepo) FindUser(_ context.Context, u string) (string, string, bool, error) {
	if u != "admin" {
		return "", "", false, errors.New("not found")
	}
	return "user-1", f.password, true, nil
}
func (f *fakeRepo) CreateSession(_ context.Context, _ string, t, c []byte, _ time.Time) error {
	f.tokenHash = t
	f.csrfHash = c
	return nil
}
func (f *fakeRepo) Session(_ context.Context, h []byte) (auth.Session, error) {
	if string(h) != string(f.tokenHash) {
		return auth.Session{}, errors.New("invalid")
	}
	return auth.Session{UserID: "user-1", Username: "admin", CSRFHash: f.csrfHash, ExpiresAt: time.Now().Add(time.Hour)}, nil
}
func (f *fakeRepo) RevokeSession(context.Context, []byte) error { return nil }

func TestLoginAuthAndCSRF(t *testing.T) {
	hash, err := auth.HashPassword("password-admin-kuat")
	if err != nil {
		t.Fatal(err)
	}
	repo := &fakeRepo{password: hash, readings: map[string]collectors.Reading{}}
	s := New(repo, false, nil)
	h := s.Handler()
	get := httptest.NewRecorder()
	h.ServeHTTP(get, httptest.NewRequest("GET", "/login", nil))
	if get.Code != 200 {
		t.Fatalf("GET login=%d", get.Code)
	}
	var csrf string
	for _, c := range get.Result().Cookies() {
		if c.Name == "dko_login_csrf" {
			csrf = c.Value
		}
	}
	form := url.Values{"username": {"admin"}, "password": {"password-admin-kuat"}, "csrf": {csrf}}
	req := httptest.NewRequest("POST", "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "dko_login_csrf", Value: csrf})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 303 {
		t.Fatalf("login=%d body=%s", w.Code, w.Body.String())
	}
	var sessionCookie, csrfCookie *http.Cookie
	for _, c := range w.Result().Cookies() {
		if c.Name == auth.CookieName {
			sessionCookie = c
		}
		if c.Name == "dko_csrf" {
			csrfCookie = c
		}
	}
	if sessionCookie == nil || !sessionCookie.HttpOnly || sessionCookie.SameSite != http.SameSiteStrictMode {
		t.Fatal("cookie sesi tidak aman")
	}
	api := httptest.NewRequest("GET", "/api/v1/providers", nil)
	api.AddCookie(sessionCookie)
	aw := httptest.NewRecorder()
	h.ServeHTTP(aw, api)
	if aw.Code != 200 {
		t.Fatalf("API=%d", aw.Code)
	}
	logout := httptest.NewRequest("POST", "/logout", strings.NewReader("csrf=salah"))
	logout.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	logout.AddCookie(sessionCookie)
	logout.AddCookie(csrfCookie)
	lw := httptest.NewRecorder()
	h.ServeHTTP(lw, logout)
	if lw.Code != 403 {
		t.Fatalf("CSRF logout=%d", lw.Code)
	}
}

func TestUnauthenticatedRedirect(t *testing.T) {
	s := New(&fakeRepo{}, false, nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/providers", nil))
	if w.Code != 303 {
		t.Fatalf("want 303 got %d", w.Code)
	}
}

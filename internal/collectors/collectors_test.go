package collectors

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRequestResponses(t *testing.T) {
	tests := []struct {
		name             string
		status           int
		body, want, kind string
	}{{"sukses", 200, `{"data":{"balance":12.5}}`, "", ""}, {"gagal", 500, `{"error":"x"}`, "HTTP 500", "upstream"}, {"malformed", 200, `{`, "format respons", "malformed"}, {"auth", 401, `{}`, "autentikasi ditolak", "auth"}}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()
			h := httpClient{client: &http.Client{Timeout: time.Second}}
			_, err := h.request(context.Background(), "GET", srv.URL, nil, nil)
			if tc.want == "" && err != nil {
				t.Fatal(err)
			}
			if tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want)) {
				t.Fatalf("want %q got %v", tc.want, err)
			}
			if tc.kind != "" {
				var pe providerError
				if !errors.As(err, &pe) || pe.kind != tc.kind {
					t.Fatalf("klasifikasi=%q, want %q", pe.kind, tc.kind)
				}
			}
		})
	}
}

func TestRequestTimeoutIsSanitized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { time.Sleep(50 * time.Millisecond) }))
	defer srv.Close()
	h := httpClient{client: &http.Client{Timeout: 5 * time.Millisecond}}
	_, err := h.request(context.Background(), "GET", srv.URL+"/jangan-bocor", nil, nil)
	var pe providerError
	if !errors.As(err, &pe) || pe.kind != "timeout" || strings.Contains(err.Error(), "jangan-bocor") {
		t.Fatalf("error timeout tidak aman: %v", err)
	}
}
func TestRedirectNotFollowed(t *testing.T) {
	targetHit := false
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { targetHit = true }))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, http.StatusFound) }))
	defer source.Close()
	h := newClient()
	_, err := h.request(context.Background(), "GET", source.URL, nil, nil)
	if err == nil || targetHit {
		t.Fatal("redirect tidak boleh diikuti")
	}
}

func TestCredentialFactory(t *testing.T) {
	tests := []struct {
		name, provider string
		values         map[string]string
		ok             bool
	}{{"proxyrack", "proxyrack", map[string]string{"api_key": "x"}, true}, {"repocket-invalid", "repocket", map[string]string{"email": "a@b"}, false}, {"provider-invalid", "asing", map[string]string{"token": "x"}, false}}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c, err := FromCredential("account-1", tc.provider, tc.values)
			if tc.ok && (err != nil || !c.Enabled() || c.ID() != "account-1") {
				t.Fatalf("factory gagal: %v", err)
			}
			if !tc.ok && err == nil {
				t.Fatal("credential invalid diterima")
			}
		})
	}
}

func TestReadingIdentityOnSuccessAndFailure(t *testing.T) {
	for _, id := range []string{"proxyrack", "uuid-a", "uuid-b"} {
		for _, err := range []error{nil, classified("auth", "autentikasi ditolak")} {
			r := result(id, "proxyrack", "1.00", err)
			want := id
			if id == "proxyrack" {
				want = ""
			}
			if r.AccountID != want || r.Provider != "proxyrack" {
				t.Fatalf("identitas salah: %+v", r)
			}
		}
	}
}

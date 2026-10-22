package collectors

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRequestResponses(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		body, want string
	}{{"sukses", 200, `{"data":{"balance":12.5}}`, ""}, {"gagal", 500, `{"error":"x"}`, "HTTP 500"}, {"malformed", 200, `{`, "format respons"}, {"auth", 401, `{}`, "autentikasi ditolak"}}
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
		})
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

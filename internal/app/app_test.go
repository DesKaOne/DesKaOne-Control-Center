package app

import (
	"encoding/json"
	"github.com/DesKaOne/DesKaOne-Control-Center/internal/collectors"
	"net/http/httptest"
	"testing"
)

func TestAuth(t *testing.T) {
	s := New(t.TempDir()+"/cache.json", "test-secret", []collectors.Collector{collectors.NewProxyRack()})
	h := s.Handler()
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/providers", nil)
	h.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Fatalf("want 401 got %d", w.Code)
	}
	req.Header.Set("Authorization", "Bearer test-secret")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("want 200 got %d", w.Code)
	}
	var out []collectors.Reading
	if e := json.Unmarshal(w.Body.Bytes(), &out); e != nil || len(out) != 1 {
		t.Fatalf("invalid response: %v", e)
	}
}

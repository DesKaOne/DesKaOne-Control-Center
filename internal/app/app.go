package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/DesKaOne/DesKaOne-Control-Center/internal/collectors"
	"github.com/DesKaOne/DesKaOne-Control-Center/internal/config"
	"html/template"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Server struct {
	mu          sync.RWMutex
	data        map[string]collectors.Reading
	list        []collectors.Collector
	path, token string
	refreshing  bool
}

func New(path, token string, list []collectors.Collector) *Server {
	s := &Server{data: map[string]collectors.Reading{}, path: path, token: token, list: list}
	s.load()
	return s
}
func (s *Server) load() {
	b, e := os.ReadFile(s.path)
	if e == nil {
		_ = json.Unmarshal(b, &s.data)
	}
}
func (s *Server) save() {
	s.mu.RLock()
	b, e := json.MarshalIndent(s.data, "", "  ")
	s.mu.RUnlock()
	if e != nil {
		return
	}
	if e := os.MkdirAll(filepath.Dir(s.path), 0700); e != nil {
		log.Print(e)
		return
	}
	tmp := s.path + ".tmp"
	if e := os.WriteFile(tmp, b, 0600); e != nil {
		log.Print(e)
		return
	}
	if e := os.Rename(tmp, s.path); e != nil {
		log.Print(e)
	}
}
func (s *Server) Refresh(ctx context.Context) {
	s.mu.Lock()
	if s.refreshing {
		s.mu.Unlock()
		return
	}
	s.refreshing = true
	s.mu.Unlock()
	defer func() { s.mu.Lock(); s.refreshing = false; s.mu.Unlock() }()
	for _, c := range s.list {
		if !c.Enabled() {
			continue
		}
		runctx, cancel := context.WithTimeout(ctx, 20*time.Second)
		r := c.Collect(runctx)
		cancel()
		s.mu.Lock()
		if r.State != "ok" {
			if prev, found := s.data[c.ID()]; found && prev.Balance != "" {
				r.Balance = prev.Balance
				r.Currency = prev.Currency
				r.Message += " (saldo terakhir dipertahankan)"
			}
		}
		s.data[c.ID()] = r
		s.mu.Unlock()
	}
	s.save()
}
func (s *Server) Rows() []collectors.Reading {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows := make([]collectors.Reading, 0, len(s.list))
	for _, c := range s.list {
		r, ok := s.data[c.ID()]
		if !ok {
			r = collectors.Reading{Provider: c.ID(), State: "belum diperiksa"}
			if !c.Enabled() {
				r.State = "belum dikonfigurasi"
			}
		}
		rows = append(rows, r)
	}
	return rows
}
func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.token != "" && r.Header.Get("Authorization") != "Bearer "+s.token {
			http.Error(w, "Akses ditolak", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

const page = `<!doctype html><html lang="id"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta http-equiv="refresh" content="60"><title>DesKaOne Control Center</title><style>body{font-family:system-ui,sans-serif;background:#101827;color:#e9eef5;max-width:1000px;margin:auto;padding:2rem}h1{font-size:1.8rem}p,small{color:#aebbc8}.panel{background:#192638;border:1px solid #334356;border-radius:14px;padding:1.4rem;margin-top:1rem;overflow-x:auto}table{border-collapse:collapse;width:100%}th,td{padding:12px;text-align:left;border-bottom:1px solid #334356}th{color:#aebbc8}a{color:#79d6ce}.ok{color:#7be0aa}.error{color:#ffba94}</style></head><body><h1>DesKaOne Control Center</h1><p>Dasbor pemantauan terpadu · Prototipe Go · Bahasa Indonesia</p><div class="panel"><h2>Ringkasan penyedia</h2><table><tr><th>Penyedia</th><th>Saldo terakhir (USD)</th><th>Status</th><th>Terakhir diperiksa (UTC)</th></tr>{{range .}}<tr><td>{{.Provider}}</td><td>{{if .Balance}}{{.Balance}}{{else}}—{{end}}</td><td>{{.State}}{{if .Message}}<br><small>{{.Message}}</small>{{end}}</td><td>{{if .CheckedAt.IsZero}}—{{else}}{{.CheckedAt.Format "02 Jan 2006 15:04:05"}}{{end}}</td></tr>{{end}}</table></div><div class="panel"><p>Saldo adalah snapshot, bukan pendapatan periode. Tidak ada request API langsung dari browser. Penyedia lain akan masuk lewat adapter berikutnya.</p><p><a href="/api/v1/providers">Lihat JSON API</a></p></div></body></html>`

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	tpl := template.Must(template.New("dashboard").Parse(page))
	mux.HandleFunc("GET /", s.auth(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = tpl.Execute(w, s.Rows())
	}))
	mux.HandleFunc("GET /api/v1/providers", s.auth(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(s.Rows())
	}))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200); _, _ = w.Write([]byte("ok")) })
	return mux
}
func Run() error {
	if err := config.LoadDotEnv(".env"); err != nil {
		return err
	}
	addr := config.Get("DKO_LISTEN_ADDR", "127.0.0.1:8095")
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return err
	}
	token := os.Getenv("DKO_ADMIN_TOKEN")
	if token == "" && !strings.EqualFold(host, "localhost") && host != "127.0.0.1" && host != "::1" {
		return errors.New("DKO_ADMIN_TOKEN wajib untuk listen non-loopback")
	}
	s := New(config.Get("DKO_DATA_FILE", "data/snapshots.json"), token, collectors.Default())
	ctx := context.Background()
	go s.Refresh(ctx)
	go func() {
		ticker := time.NewTicker(time.Duration(config.PollMinutes()) * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			s.Refresh(ctx)
		}
	}()
	log.Printf("DesKaOne Control Center di http://%s", addr)
	return fmt.Errorf("HTTP server: %w", http.ListenAndServe(addr, s.Handler()))
}

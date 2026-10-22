package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/DesKaOne/DesKaOne-Control-Center/internal/auth"
	"github.com/DesKaOne/DesKaOne-Control-Center/internal/collectors"
	"github.com/DesKaOne/DesKaOne-Control-Center/internal/config"
	"github.com/DesKaOne/DesKaOne-Control-Center/internal/database"
)

type Repository interface {
	EnsureProviders(context.Context, []collectors.Collector) error
	RecordReading(context.Context, collectors.Reading, string) error
	LatestReadings(context.Context) (map[string]collectors.Reading, error)
	FindUser(context.Context, string) (string, string, bool, error)
	CreateSession(context.Context, string, []byte, []byte, time.Time) error
	Session(context.Context, []byte) (auth.Session, error)
	RevokeSession(context.Context, []byte) error
}

type Server struct {
	repo         Repository
	list         []collectors.Collector
	secureCookie bool
	limiter      *auth.Limiter
}

func New(repo Repository, secureCookie bool, list []collectors.Collector) *Server {
	return &Server{repo: repo, secureCookie: secureCookie, list: list, limiter: auth.NewLimiter(5, 15*time.Minute)}
}

func (s *Server) Refresh(ctx context.Context) {
	for _, c := range s.list {
		if !c.Enabled() {
			continue
		}
		runCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		r := c.Collect(runCtx)
		cancel()
		if err := s.repo.RecordReading(ctx, r, r.CheckedAt.UTC().Format(time.RFC3339Nano)); err != nil {
			log.Printf("gagal menyimpan hasil collector %s: %v", c.ID(), err)
		}
	}
}

func (s *Server) Rows(ctx context.Context) ([]collectors.Reading, error) {
	data, err := s.repo.LatestReadings(ctx)
	if err != nil {
		return nil, err
	}
	rows := make([]collectors.Reading, 0, len(s.list))
	for _, c := range s.list {
		r, ok := data[c.ID()]
		if !ok {
			r = collectors.Reading{Provider: c.ID(), State: "belum diperiksa"}
			if !c.Enabled() {
				r.State = "belum dikonfigurasi"
			}
		}
		rows = append(rows, r)
	}
	return rows, nil
}

type pageData struct {
	Rows           []collectors.Reading
	CSRF, Username string
}

const dashboardPage = `<!doctype html><html lang="id"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta http-equiv="refresh" content="60"><title>DesKaOne Control Center</title><style>body{font-family:system-ui,sans-serif;background:#101827;color:#e9eef5;max-width:1000px;margin:auto;padding:2rem}p,small{color:#aebbc8}.panel{background:#192638;border:1px solid #334356;border-radius:14px;padding:1.4rem;margin-top:1rem;overflow-x:auto}table{border-collapse:collapse;width:100%}th,td{padding:12px;text-align:left;border-bottom:1px solid #334356}a{color:#79d6ce}button{padding:.6rem 1rem}</style></head><body><h1>DesKaOne Control Center</h1><p>Dasbor pemantauan terpadu · Masuk sebagai {{.Username}}</p><form method="post" action="/logout"><input type="hidden" name="csrf" value="{{.CSRF}}"><button type="submit">Keluar</button></form><div class="panel"><h2>Ringkasan penyedia</h2><table><tr><th>Penyedia</th><th>Saldo terakhir</th><th>Status</th><th>Terakhir diperiksa (UTC)</th></tr>{{range .Rows}}<tr><td>{{.Provider}}</td><td>{{if .Balance}}{{.Balance}} {{.Currency}}{{else}}—{{end}}</td><td>{{.State}}{{if .Message}}<br><small>{{.Message}}</small>{{end}}</td><td>{{if .CheckedAt.IsZero}}—{{else}}{{.CheckedAt.Format "02 Jan 2006 15:04:05"}}{{end}}</td></tr>{{end}}</table></div><div class="panel"><p>Saldo adalah snapshot, bukan pendapatan terverifikasi. Total pendapatan tidak dihitung dari penjumlahan saldo.</p><p><a href="/api/v1/providers">Lihat JSON API</a></p></div></body></html>`
const loginPage = `<!doctype html><html lang="id"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Masuk</title><style>body{font-family:system-ui;background:#101827;color:#e9eef5;max-width:420px;margin:4rem auto;padding:2rem}form{display:grid;gap:1rem}input,button{padding:.8rem}</style></head><body><h1>Masuk Admin</h1>{{if .Error}}<p>Nama pengguna atau kata sandi salah.</p>{{end}}<form method="post" action="/login"><input type="hidden" name="csrf" value="{{.CSRF}}"><label>Nama pengguna<input name="username" autocomplete="username" required></label><label>Kata sandi<input type="password" name="password" autocomplete="current-password" required></label><button type="submit">Masuk</button></form></body></html>`

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'unsafe-inline'")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	dash := template.Must(template.New("dashboard").Parse(dashboardPage))
	login := template.Must(template.New("login").Parse(loginPage))
	mux.HandleFunc("GET /login", func(w http.ResponseWriter, r *http.Request) {
		raw, _, err := auth.NewToken()
		if err != nil {
			http.Error(w, "Layanan tidak tersedia", 500)
			return
		}
		s.setCookie(w, "dko_login_csrf", raw, false, 10*time.Minute)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = login.Execute(w, map[string]any{"CSRF": raw, "Error": r.URL.Query().Get("gagal") != ""})
	})
	mux.HandleFunc("POST /login", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Permintaan tidak valid", 400)
			return
		}
		csrfCookie, err := r.Cookie("dko_login_csrf")
		if err != nil || !auth.ValidCSRF(r.FormValue("csrf"), auth.HashToken(csrfCookie.Value)) {
			http.Error(w, "CSRF tidak valid", 403)
			return
		}
		key := auth.ParseForwardedIP(r.RemoteAddr)
		if !s.limiter.Allow(key, time.Now()) {
			http.Error(w, "Terlalu banyak percobaan. Coba lagi nanti.", 429)
			return
		}
		id, hash, active, err := s.repo.FindUser(r.Context(), strings.ToLower(strings.TrimSpace(r.FormValue("username"))))
		if err != nil || !active || !auth.VerifyPassword(hash, r.FormValue("password")) {
			http.Redirect(w, r, "/login?gagal=1", http.StatusSeeOther)
			return
		}
		raw, tokenHash, err := auth.NewToken()
		if err != nil {
			http.Error(w, "Layanan tidak tersedia", 500)
			return
		}
		csrf, csrfHash, err := auth.NewToken()
		if err != nil {
			http.Error(w, "Layanan tidak tersedia", 500)
			return
		}
		expires := time.Now().Add(12 * time.Hour)
		if err = s.repo.CreateSession(r.Context(), id, tokenHash, csrfHash, expires); err != nil {
			http.Error(w, "Layanan tidak tersedia", 500)
			return
		}
		s.limiter.Reset(key)
		s.setCookie(w, auth.CookieName, raw, true, time.Until(expires))
		s.setCookie(w, "dko_csrf", csrf, false, time.Until(expires))
		http.Redirect(w, r, "/", http.StatusSeeOther)
	})
	mux.HandleFunc("POST /logout", s.requireSession(true, func(w http.ResponseWriter, r *http.Request, _ auth.Session) {
		c, _ := r.Cookie(auth.CookieName)
		_ = s.repo.RevokeSession(r.Context(), auth.HashToken(c.Value))
		s.clearCookie(w, auth.CookieName, true)
		s.clearCookie(w, "dko_csrf", false)
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	}))
	mux.HandleFunc("GET /", s.requireSession(false, func(w http.ResponseWriter, r *http.Request, session auth.Session) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		rows, err := s.Rows(r.Context())
		if err != nil {
			http.Error(w, "Data tidak tersedia", 500)
			return
		}
		csrf, _ := r.Cookie("dko_csrf")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = dash.Execute(w, pageData{Rows: rows, CSRF: csrf.Value, Username: session.Username})
	}))
	mux.HandleFunc("GET /api/v1/providers", s.requireSession(false, func(w http.ResponseWriter, r *http.Request, _ auth.Session) {
		rows, err := s.Rows(r.Context())
		if err != nil {
			http.Error(w, "Data tidak tersedia", 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(rows)
	}))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200); _, _ = w.Write([]byte("ok")) })
	return securityHeaders(mux)
}
func (s *Server) requireSession(csrfRequired bool, next func(http.ResponseWriter, *http.Request, auth.Session)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(auth.CookieName)
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		session, err := s.repo.Session(r.Context(), auth.HashToken(c.Value))
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		if csrfRequired {
			if err := r.ParseForm(); err != nil || !auth.ValidCSRF(r.FormValue("csrf"), session.CSRFHash) {
				http.Error(w, "CSRF tidak valid", 403)
				return
			}
		}
		next(w, r, session)
	}
}
func (s *Server) setCookie(w http.ResponseWriter, name, value string, httpOnly bool, maxAge time.Duration) {
	http.SetCookie(w, &http.Cookie{Name: name, Value: value, Path: "/", HttpOnly: httpOnly, Secure: s.secureCookie, SameSite: http.SameSiteStrictMode, MaxAge: int(maxAge.Seconds())})
}
func (s *Server) clearCookie(w http.ResponseWriter, name string, httpOnly bool) {
	http.SetCookie(w, &http.Cookie{Name: name, Value: "", Path: "/", HttpOnly: httpOnly, Secure: s.secureCookie, SameSite: http.SameSiteStrictMode, MaxAge: -1})
}

func Run() error {
	if err := config.LoadDotEnv(".env"); err != nil {
		return err
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	store, err := database.Open(ctx, cfg.DatabaseURL, cfg.DatabaseTimeout)
	if err != nil {
		return err
	}
	defer store.Close()
	if username, password := os.Getenv("DKO_BOOTSTRAP_ADMIN_USERNAME"), os.Getenv("DKO_BOOTSTRAP_ADMIN_PASSWORD"); username != "" || password != "" {
		if username == "" || password == "" {
			return errors.New("bootstrap admin memerlukan username dan password")
		}
		hash, hashErr := auth.HashPassword(password)
		if hashErr != nil {
			return fmt.Errorf("bootstrap admin: %w", hashErr)
		}
		if err := store.UpsertBootstrapAdmin(ctx, username, hash); err != nil {
			return fmt.Errorf("bootstrap admin: %w", err)
		}
	}
	list := collectors.Default()
	if err := store.EnsureProviders(ctx, list); err != nil {
		return fmt.Errorf("inisialisasi provider: %w", err)
	}
	server := New(store, cfg.CookieSecure, list)
	go server.Refresh(ctx)
	go func() {
		ticker := time.NewTicker(cfg.PollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				server.Refresh(ctx)
			}
		}
	}()
	httpServer := &http.Server{Addr: cfg.ListenAddr, Handler: server.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	errCh := make(chan error, 1)
	go func() {
		log.Printf("DesKaOne Control Center siap di %s", cfg.ListenAddr)
		errCh <- httpServer.ListenAndServe()
	}()
	select {
	case err = <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("HTTP server: %w", err)
		}
		return nil
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return httpServer.Shutdown(shutdownCtx)
	}
}

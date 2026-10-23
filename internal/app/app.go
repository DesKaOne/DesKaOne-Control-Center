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
	"sync"
	"syscall"
	"time"

	"github.com/DesKaOne/DesKaOne-Control-Center/internal/auth"
	"github.com/DesKaOne/DesKaOne-Control-Center/internal/collectors"
	"github.com/DesKaOne/DesKaOne-Control-Center/internal/config"
	"github.com/DesKaOne/DesKaOne-Control-Center/internal/credentials"
	"github.com/DesKaOne/DesKaOne-Control-Center/internal/database"
	"github.com/DesKaOne/DesKaOne-Control-Center/internal/providers"
)

type Repository interface {
	EnsureProviders(context.Context, []collectors.Collector) error
	RecordReading(context.Context, collectors.Reading, string) error
	LatestReadings(context.Context) (map[string]collectors.Reading, error)
	FindUser(context.Context, string) (string, string, bool, error)
	CreateSession(context.Context, string, []byte, []byte, time.Time) error
	Session(context.Context, []byte) (auth.Session, error)
	RevokeSession(context.Context, []byte) error
	ListProviderAccounts(context.Context) ([]providers.Account, error)
	CreateProviderAccount(context.Context, string, string, string) (string, error)
	UpdateProviderAccount(context.Context, string, string, bool) error
	Audit(context.Context, string, string, string) error
}

type CredentialService interface {
	Get(context.Context, string) (map[string]string, error)
	Put(context.Context, string, map[string]string) error
}

type Server struct {
	repo         Repository
	list         []collectors.Collector
	secureCookie bool
	limiter      *auth.Limiter
	credentials  CredentialService
	mu           sync.RWMutex
}

func New(repo Repository, secureCookie bool, list []collectors.Collector) *Server {
	return &Server{repo: repo, secureCookie: secureCookie, list: list, limiter: auth.NewLimiter(5, 15*time.Minute)}
}

func NewManaged(repo Repository, secureCookie bool, service CredentialService) *Server {
	return &Server{repo: repo, secureCookie: secureCookie, credentials: service, limiter: auth.NewLimiter(5, 15*time.Minute)}
}

func (s *Server) reloadCollectors(ctx context.Context) error {
	if s.credentials == nil {
		return nil
	}
	accounts, err := s.repo.ListProviderAccounts(ctx)
	if err != nil {
		return err
	}
	list := make([]collectors.Collector, 0, len(accounts))
	for _, account := range accounts {
		if !account.Enabled {
			continue
		}
		if account.CredentialStored {
			values, getErr := s.credentials.Get(ctx, account.ID)
			if getErr != nil {
				continue
			}
			collector, makeErr := collectors.FromCredential(account.ID, account.ProviderSlug, values)
			if makeErr == nil {
				list = append(list, collector)
			}
		} else if account.AccountKey == "default" {
			collector, makeErr := collectors.FromLegacy(account.ID, account.ProviderSlug)
			if makeErr == nil {
				list = append(list, collector)
			}
		}
	}
	s.mu.Lock()
	s.list = list
	s.mu.Unlock()
	return nil
}

func (s *Server) Refresh(ctx context.Context) {
	_ = s.reloadCollectors(ctx)
	s.mu.RLock()
	list := append([]collectors.Collector(nil), s.list...)
	s.mu.RUnlock()
	for _, c := range list {
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
	accounts, err := s.repo.ListProviderAccounts(ctx)
	if err != nil {
		return nil, err
	}
	rows := make([]collectors.Reading, 0, len(accounts))
	for _, account := range accounts {
		r, ok := data[account.ID]
		if !ok {
			r = collectors.Reading{State: "belum diperiksa"}
			if !account.Enabled {
				r.State = "dinonaktifkan"
			} else if !account.CredentialStored {
				r.State = "credential belum tersimpan"
			}
		}
		r.AccountID = account.ID
		r.Provider = account.DisplayName
		rows = append(rows, r)
	}
	return rows, nil
}

type pageData struct {
	Rows           []collectors.Reading
	CSRF, Username string
}

const dashboardPage = `<!doctype html><html lang="id"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta http-equiv="refresh" content="60"><title>DesKaOne Control Center</title><style>body{font-family:system-ui,sans-serif;background:#101827;color:#e9eef5;max-width:1000px;margin:auto;padding:2rem}p,small{color:#aebbc8}.panel{background:#192638;border:1px solid #334356;border-radius:14px;padding:1.4rem;margin-top:1rem;overflow-x:auto}table{border-collapse:collapse;width:100%}th,td{padding:12px;text-align:left;border-bottom:1px solid #334356}a{color:#79d6ce}button{padding:.6rem 1rem}</style></head><body><h1>DesKaOne Control Center</h1><p>Dasbor pemantauan terpadu · Masuk sebagai {{.Username}}</p><p><a href="/admin/providers">Kelola akun provider</a></p><form method="post" action="/logout"><input type="hidden" name="csrf" value="{{.CSRF}}"><button type="submit">Keluar</button></form><div class="panel"><h2>Ringkasan penyedia</h2><table><tr><th>Penyedia</th><th>Saldo terakhir</th><th>Status</th><th>Terakhir diperiksa (UTC)</th></tr>{{range .Rows}}<tr><td>{{.Provider}}</td><td>{{if .Balance}}{{.Balance}} {{.Currency}}{{else}}—{{end}}</td><td>{{.State}}{{if .Message}}<br><small>{{.Message}}</small>{{end}}</td><td>{{if .CheckedAt.IsZero}}—{{else}}{{.CheckedAt.Format "02 Jan 2006 15:04:05"}}{{end}}</td></tr>{{end}}</table></div><div class="panel"><p>Saldo adalah snapshot, bukan pendapatan terverifikasi. Total pendapatan tidak dihitung dari penjumlahan saldo.</p><p><a href="/api/v1/providers">Lihat JSON API</a></p></div></body></html>`
const loginPage = `<!doctype html><html lang="id"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Masuk</title><style>body{font-family:system-ui;background:#101827;color:#e9eef5;max-width:420px;margin:4rem auto;padding:2rem}form{display:grid;gap:1rem}input,button{padding:.8rem}</style></head><body><h1>Masuk Admin</h1>{{if .Error}}<p>Nama pengguna atau kata sandi salah.</p>{{end}}<form method="post" action="/login"><input type="hidden" name="csrf" value="{{.CSRF}}"><label>Nama pengguna<input name="username" autocomplete="username" required></label><label>Kata sandi<input type="password" name="password" autocomplete="current-password" required></label><button type="submit">Masuk</button></form></body></html>`
const providersPage = `<!doctype html><html lang="id"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Kelola Provider</title><style>body{font-family:system-ui;background:#101827;color:#e9eef5;max-width:1000px;margin:auto;padding:2rem}.panel{background:#192638;padding:1rem;margin:1rem 0;border-radius:12px}form{display:grid;gap:.6rem}input,select,button{padding:.6rem}.status{color:#7be0aa}</style></head><body><p><a href="/">← Dasbor</a></p><h1>Kelola Akun Provider</h1><p>Credential tidak pernah ditampilkan kembali. Status <strong>Tersimpan</strong> hanya menunjukkan bahwa ciphertext tersedia.</p>{{range .Accounts}}<div class="panel"><h2>{{.DisplayName}}</h2><p>{{.ProviderSlug}} / {{.AccountKey}} · Credential: {{if .CredentialStored}}<span class="status">Tersimpan</span>{{else}}Belum tersimpan{{end}}</p><form method="post" action="/admin/providers/update"><input type="hidden" name="csrf" value="{{$.CSRF}}"><input type="hidden" name="id" value="{{.ID}}"><label>Nama tampilan<input name="display_name" value="{{.DisplayName}}" maxlength="100" required></label><label><input type="checkbox" name="enabled" value="true" {{if .Enabled}}checked{{end}}> Aktif</label><label>API key baru (ProxyRack)<input type="password" name="api_key" autocomplete="new-password"></label><label>Token baru (TraffMonetizer)<input type="password" name="token" autocomplete="new-password"></label><label>Email baru (Repocket)<input type="email" name="email" autocomplete="off"></label><label>Password baru (Repocket)<input type="password" name="provider_password" autocomplete="new-password"></label><button>Simpan perubahan</button></form></div>{{end}}<div class="panel"><h2>Tambah akun</h2><form method="post" action="/admin/providers/create"><input type="hidden" name="csrf" value="{{.CSRF}}"><label>Provider<select name="provider"><option value="proxyrack">ProxyRack</option><option value="traffmonetizer">TraffMonetizer</option><option value="repocket">Repocket</option></select></label><label>Kunci akun<input name="account_key" pattern="[a-z0-9][a-z0-9_-]{0,62}" maxlength="63" required></label><label>Nama tampilan<input name="display_name" maxlength="100" required></label><label>API key (ProxyRack)<input type="password" name="api_key" autocomplete="new-password"></label><label>Token (TraffMonetizer)<input type="password" name="token" autocomplete="new-password"></label><label>Email (Repocket)<input type="email" name="email" autocomplete="off"></label><label>Password (Repocket)<input type="password" name="provider_password" autocomplete="new-password"></label><button>Tambah akun</button></form></div></body></html>`

func credentialForm(r *http.Request, provider string) map[string]string {
	switch provider {
	case "proxyrack":
		return map[string]string{"api_key": r.FormValue("api_key")}
	case "traffmonetizer":
		return map[string]string{"token": r.FormValue("token")}
	case "repocket":
		return map[string]string{"email": r.FormValue("email"), "password": r.FormValue("provider_password")}
	default:
		return nil
	}
}

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
	providerTemplate := template.Must(template.New("providers").Parse(providersPage))
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
		r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
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
	mux.HandleFunc("GET /admin/providers", s.requireSession(false, func(w http.ResponseWriter, r *http.Request, _ auth.Session) {
		accounts, err := s.repo.ListProviderAccounts(r.Context())
		if err != nil {
			http.Error(w, "Data akun tidak tersedia", 500)
			return
		}
		csrf, err := r.Cookie("dko_csrf")
		if err != nil {
			http.Error(w, "Sesi tidak valid", 403)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = providerTemplate.Execute(w, map[string]any{"Accounts": accounts, "CSRF": csrf.Value})
	}))
	mux.HandleFunc("POST /admin/providers/create", s.requireSession(true, func(w http.ResponseWriter, r *http.Request, session auth.Session) {
		if s.credentials == nil {
			http.Error(w, "Vault tidak tersedia", 503)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
		slug := strings.ToLower(providers.Clean(r.FormValue("provider")))
		key := strings.ToLower(providers.Clean(r.FormValue("account_key")))
		name := providers.Clean(r.FormValue("display_name"))
		values := credentialForm(r, slug)
		if !providers.ValidSlug(slug) || !providers.ValidAccountKey(key) || name == "" || len(name) > 100 {
			http.Error(w, "Data akun tidak valid", 400)
			return
		}
		if _, err := collectors.FromCredential("validasi", slug, values); err != nil {
			http.Error(w, "Credential provider tidak lengkap", 400)
			return
		}
		id, err := s.repo.CreateProviderAccount(r.Context(), slug, key, name)
		if err != nil {
			http.Error(w, "Akun provider tidak dapat dibuat", 400)
			return
		}
		if err = s.credentials.Put(r.Context(), id, values); err != nil {
			http.Error(w, "Credential tidak dapat disimpan", 500)
			return
		}
		if err = s.repo.UpdateProviderAccount(r.Context(), id, name, true); err != nil {
			http.Error(w, "Akun provider tidak dapat diaktifkan", 500)
			return
		}
		_ = s.repo.Audit(r.Context(), session.UserID, "provider_account.create", id)
		_ = s.reloadCollectors(r.Context())
		http.Redirect(w, r, "/admin/providers", 303)
	}))
	mux.HandleFunc("POST /admin/providers/update", s.requireSession(true, func(w http.ResponseWriter, r *http.Request, session auth.Session) {
		r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
		id := providers.Clean(r.FormValue("id"))
		name := providers.Clean(r.FormValue("display_name"))
		if id == "" || name == "" || len(name) > 100 {
			http.Error(w, "Data akun tidak valid", 400)
			return
		}
		accounts, err := s.repo.ListProviderAccounts(r.Context())
		if err != nil {
			http.Error(w, "Data akun tidak tersedia", 500)
			return
		}
		var account *providers.Account
		for i := range accounts {
			if accounts[i].ID == id {
				account = &accounts[i]
				break
			}
		}
		if account == nil {
			http.Error(w, "Akun provider tidak ditemukan", 404)
			return
		}
		values := credentialForm(r, account.ProviderSlug)
		hasNew := false
		for _, v := range values {
			if v != "" {
				hasNew = true
			}
		}
		if hasNew {
			if _, err := collectors.FromCredential(id, account.ProviderSlug, values); err != nil {
				http.Error(w, "Credential baru tidak lengkap", 400)
				return
			}
			if err = s.credentials.Put(r.Context(), id, values); err != nil {
				http.Error(w, "Credential tidak dapat disimpan", 500)
				return
			}
		}
		enabled := r.FormValue("enabled") == "true"
		if enabled && !account.CredentialStored && !hasNew {
			http.Error(w, "Simpan credential sebelum mengaktifkan akun", 400)
			return
		}
		if err = s.repo.UpdateProviderAccount(r.Context(), id, name, enabled); err != nil {
			http.Error(w, "Perubahan tidak dapat disimpan", 500)
			return
		}
		_ = s.repo.Audit(r.Context(), session.UserID, "provider_account.update", id)
		_ = s.reloadCollectors(r.Context())
		http.Redirect(w, r, "/admin/providers", 303)
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
			r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
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
	credentialService, err := credentials.New(store, cfg.MasterKey)
	if err != nil {
		return fmt.Errorf("credential vault: %w", err)
	}
	server := NewManaged(store, cfg.CookieSecure, credentialService)
	if err := server.reloadCollectors(ctx); err != nil {
		return fmt.Errorf("memuat akun provider: %w", err)
	}
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

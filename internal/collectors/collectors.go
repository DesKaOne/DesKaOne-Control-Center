package collectors

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Reading struct {
	Provider  string    `json:"provider"`
	Balance   string    `json:"balance,omitempty"`
	Currency  string    `json:"currency,omitempty"`
	State     string    `json:"state"`
	Message   string    `json:"message,omitempty"`
	CheckedAt time.Time `json:"checked_at"`
}
type Collector interface {
	ID() string
	Enabled() bool
	Collect(context.Context) Reading
}
type httpClient struct{ client *http.Client }

func newClient() httpClient {
	return httpClient{client: &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}}
}
func (h httpClient) request(ctx context.Context, method, endpoint string, headers map[string]string, body io.Reader) (map[string]any, error) {
	req, e := http.NewRequestWithContext(ctx, method, endpoint, body)
	if e != nil {
		return nil, e
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	res, e := h.client.Do(req)
	if e != nil {
		return nil, e
	}
	defer res.Body.Close()
	if res.StatusCode == 401 || res.StatusCode == 403 {
		return nil, errors.New("autentikasi ditolak atau token kedaluwarsa")
	}
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return nil, fmt.Errorf("penyedia mengembalikan HTTP %d", res.StatusCode)
	}
	var obj map[string]any
	if e := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&obj); e != nil {
		return nil, errors.New("format respons API tidak valid")
	}
	return obj, nil
}
func value(obj map[string]any, keys ...string) (any, bool) {
	var cur any = obj
	for _, k := range keys {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		cur, ok = m[k]
		if !ok {
			return nil, false
		}
	}
	return cur, true
}
func numberString(v any) (string, error) {
	switch n := v.(type) {
	case float64:
		return strconv.FormatFloat(n, 'f', 4, 64), nil
	case string:
		s := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(n), "$"))
		if _, err := strconv.ParseFloat(s, 64); err != nil {
			return "", errors.New("format saldo tidak valid")
		}
		return s, nil
	default:
		return "", errors.New("nilai saldo tidak tersedia")
	}
}
func result(id string, raw any, err error) Reading {
	r := Reading{Provider: id, Currency: "USD", CheckedAt: time.Now().UTC()}
	if err != nil {
		r.State = "error"
		r.Message = err.Error()
		return r
	}
	b, e := numberString(raw)
	if e != nil {
		r.State = "error"
		r.Message = e.Error()
		return r
	}
	r.State = "ok"
	r.Balance = b
	return r
}

type ProxyRack struct{ httpClient }

func NewProxyRack() ProxyRack   { return ProxyRack{newClient()} }
func (ProxyRack) ID() string    { return "proxyrack" }
func (ProxyRack) Enabled() bool { return os.Getenv("DKO_PROXYRACK_API_KEY") != "" }
func (p ProxyRack) Collect(ctx context.Context) Reading {
	obj, e := p.request(ctx, "POST", "https://peer.proxyrack.com/api/balance", map[string]string{"Api-Key": os.Getenv("DKO_PROXYRACK_API_KEY"), "Accept": "application/json"}, nil)
	if e != nil {
		return result(p.ID(), nil, e)
	}
	v, ok := value(obj, "data", "balance")
	if !ok {
		return result(p.ID(), nil, errors.New("field balance tidak ditemukan"))
	}
	return result(p.ID(), v, nil)
}

type TraffMonetizer struct{ httpClient }

func NewTraffMonetizer() TraffMonetizer { return TraffMonetizer{newClient()} }
func (TraffMonetizer) ID() string       { return "traffmonetizer" }
func (TraffMonetizer) Enabled() bool    { return os.Getenv("DKO_TRAFFMONETIZER_TOKEN") != "" }
func (t TraffMonetizer) Collect(ctx context.Context) Reading {
	obj, e := t.request(ctx, "GET", "https://data.traffmonetizer.com/api/app_user/get_balance", map[string]string{"Authorization": "Bearer " + os.Getenv("DKO_TRAFFMONETIZER_TOKEN"), "Origin": "https://app.traffmonetizer.com", "Referer": "https://app.traffmonetizer.com/"}, nil)
	if e != nil {
		return result(t.ID(), nil, e)
	}
	v, ok := value(obj, "data", "balance")
	if !ok {
		return result(t.ID(), nil, errors.New("field balance tidak ditemukan"))
	}
	return result(t.ID(), v, nil)
}

type Repocket struct{ httpClient }

func NewRepocket() Repocket { return Repocket{newClient()} }
func (Repocket) ID() string { return "repocket" }
func (Repocket) Enabled() bool {
	return os.Getenv("DKO_REPOCKET_EMAIL") != "" && os.Getenv("DKO_REPOCKET_PASSWORD") != ""
}

// Authentication mechanism follows CashPilot's observed Firebase password flow; endpoint may change.
func (r Repocket) Collect(ctx context.Context) Reading {
	key := "AIzaSyBJf6hyw47O-5TrAwQszkwvDEh-Ri6q6SU" // public client identifier from reference; not an account secret
	authURL := "https://identitytoolkit.googleapis.com/v1/accounts:signInWithPassword?key=" + url.QueryEscape(key)
	payload := map[string]any{"email": os.Getenv("DKO_REPOCKET_EMAIL"), "password": os.Getenv("DKO_REPOCKET_PASSWORD"), "returnSecureToken": true}
	b, _ := json.Marshal(payload)
	obj, e := r.request(ctx, "POST", authURL, map[string]string{"Content-Type": "application/json"}, strings.NewReader(string(b)))
	if e != nil {
		return result(r.ID(), nil, e)
	}
	token, ok := value(obj, "idToken")
	if !ok || token == "" {
		return result(r.ID(), nil, errors.New("token Repocket tidak tersedia"))
	}
	obj, e = r.request(ctx, "GET", "https://api.repocket.com/api/reports/current", map[string]string{"Auth-Token": fmt.Sprint(token)}, nil)
	if e != nil {
		return result(r.ID(), nil, e)
	}
	cents, ok := value(obj, "centsCredited")
	if !ok {
		return result(r.ID(), nil, errors.New("field centsCredited tidak ditemukan"))
	}
	s, e := numberString(cents)
	if e != nil {
		return result(r.ID(), nil, e)
	}
	f, _ := strconv.ParseFloat(s, 64)
	return result(r.ID(), strconv.FormatFloat(f/100, 'f', 4, 64), nil)
}
func Default() []Collector { return []Collector{NewProxyRack(), NewTraffMonetizer(), NewRepocket()} }

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
	ErrorKind string    `json:"-"`
}

type providerError struct{ kind, message string }

func (e providerError) Error() string       { return e.message }
func classified(kind, message string) error { return providerError{kind: kind, message: message} }

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
		if errors.Is(e, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, classified("timeout", "waktu tunggu penyedia habis")
		}
		return nil, classified("network", "penyedia tidak dapat dihubungi")
	}
	defer res.Body.Close()
	if res.StatusCode == 401 || res.StatusCode == 403 {
		return nil, classified("auth", "autentikasi ditolak atau token kedaluwarsa")
	}
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return nil, classified("upstream", fmt.Sprintf("penyedia mengembalikan HTTP %d", res.StatusCode))
	}
	var obj map[string]any
	if e := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&obj); e != nil {
		return nil, classified("malformed", "format respons API tidak valid")
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
		var classifiedError providerError
		if errors.As(err, &classifiedError) {
			r.ErrorKind = classifiedError.kind
		} else {
			r.ErrorKind = "malformed"
		}
		return r
	}
	b, e := numberString(raw)
	if e != nil {
		r.State = "error"
		r.Message = e.Error()
		r.ErrorKind = "malformed"
		return r
	}
	r.State = "ok"
	r.Balance = b
	return r
}

type ProxyRack struct {
	httpClient
	id, apiKey string
}

func NewProxyRack() ProxyRack {
	return ProxyRack{httpClient: newClient(), id: "proxyrack", apiKey: os.Getenv("DKO_PROXYRACK_API_KEY")}
}
func (p ProxyRack) ID() string    { return p.id }
func (p ProxyRack) Enabled() bool { return p.apiKey != "" }
func (p ProxyRack) Collect(ctx context.Context) Reading {
	obj, e := p.request(ctx, "POST", "https://peer.proxyrack.com/api/balance", map[string]string{"Api-Key": p.apiKey, "Accept": "application/json"}, nil)
	if e != nil {
		return result(p.ID(), nil, e)
	}
	v, ok := value(obj, "data", "balance")
	if !ok {
		return result(p.ID(), nil, errors.New("field balance tidak ditemukan"))
	}
	return result(p.ID(), v, nil)
}

type TraffMonetizer struct {
	httpClient
	id, token string
}

func NewTraffMonetizer() TraffMonetizer {
	return TraffMonetizer{httpClient: newClient(), id: "traffmonetizer", token: os.Getenv("DKO_TRAFFMONETIZER_TOKEN")}
}
func (t TraffMonetizer) ID() string    { return t.id }
func (t TraffMonetizer) Enabled() bool { return t.token != "" }
func (t TraffMonetizer) Collect(ctx context.Context) Reading {
	obj, e := t.request(ctx, "GET", "https://data.traffmonetizer.com/api/app_user/get_balance", map[string]string{"Authorization": "Bearer " + t.token, "Origin": "https://app.traffmonetizer.com", "Referer": "https://app.traffmonetizer.com/"}, nil)
	if e != nil {
		return result(t.ID(), nil, e)
	}
	v, ok := value(obj, "data", "balance")
	if !ok {
		return result(t.ID(), nil, errors.New("field balance tidak ditemukan"))
	}
	return result(t.ID(), v, nil)
}

type Repocket struct {
	httpClient
	id, email, password string
}

func NewRepocket() Repocket {
	return Repocket{httpClient: newClient(), id: "repocket", email: os.Getenv("DKO_REPOCKET_EMAIL"), password: os.Getenv("DKO_REPOCKET_PASSWORD")}
}
func (r Repocket) ID() string { return r.id }
func (r Repocket) Enabled() bool {
	return r.email != "" && r.password != ""
}

// Authentication mechanism follows CashPilot's observed Firebase password flow; endpoint may change.
func (r Repocket) Collect(ctx context.Context) Reading {
	key := "AIzaSyBJf6hyw47O-5TrAwQszkwvDEh-Ri6q6SU" // public client identifier from reference; not an account secret
	authURL := "https://identitytoolkit.googleapis.com/v1/accounts:signInWithPassword?key=" + url.QueryEscape(key)
	payload := map[string]any{"email": r.email, "password": r.password, "returnSecureToken": true}
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

func FromCredential(accountID, provider string, values map[string]string) (Collector, error) {
	switch provider {
	case "proxyrack":
		if strings.TrimSpace(values["api_key"]) == "" {
			return nil, errors.New("API key wajib diisi")
		}
		return ProxyRack{httpClient: newClient(), id: accountID, apiKey: values["api_key"]}, nil
	case "traffmonetizer":
		if strings.TrimSpace(values["token"]) == "" {
			return nil, errors.New("token wajib diisi")
		}
		return TraffMonetizer{httpClient: newClient(), id: accountID, token: values["token"]}, nil
	case "repocket":
		if strings.TrimSpace(values["email"]) == "" || values["password"] == "" {
			return nil, errors.New("email dan password wajib diisi")
		}
		return Repocket{httpClient: newClient(), id: accountID, email: values["email"], password: values["password"]}, nil
	default:
		return nil, errors.New("provider tidak didukung")
	}
}

func FromLegacy(accountID, provider string) (Collector, error) {
	values := map[string]string{}
	switch provider {
	case "proxyrack":
		values["api_key"] = os.Getenv("DKO_PROXYRACK_API_KEY")
	case "traffmonetizer":
		values["token"] = os.Getenv("DKO_TRAFFMONETIZER_TOKEN")
	case "repocket":
		values["email"] = os.Getenv("DKO_REPOCKET_EMAIL")
		values["password"] = os.Getenv("DKO_REPOCKET_PASSWORD")
	}
	return FromCredential(accountID, provider, values)
}

package handlers

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/leogoesger/yakkkkk-api/internal/config"
	"github.com/leogoesger/yakkkkk-api/internal/water"
)

// Sent by clients; must never reach the upstream or the logs.
const clientAuth = "Bearer client-secret"

type seen struct {
	method, path, query, host, auth, cookie, body string
}

// setup starts a fake HTTPS upstream and the proxy in front of it.
func setup(t *testing.T, mutate func(*config.Config)) (srv *httptest.Server, got *seen, logs *bytes.Buffer) {
	t.Helper()
	got = &seen{}
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		*got = seen{r.Method, r.URL.Path, r.URL.RawQuery, r.Host, r.Header.Get("Authorization"), r.Header.Get("Cookie"), string(b)}
		http.SetCookie(w, &http.Cookie{Name: "s", Value: "x"})
		w.Write([]byte("upstream ok"))
	}))
	t.Cleanup(upstream.Close)
	target, _ := url.Parse(upstream.URL + "/v1")

	cfg := &config.Config{
		Routes:          []config.Route{{Name: "svc", Target: target, Header: "Authorization", Value: "Bearer upstream-secret"}},
		MaxBodyBytes:    1024,
		UpstreamTimeout: 5 * time.Second,
	}
	if mutate != nil {
		mutate(cfg)
	}
	tr := upstream.Client().Transport.(*http.Transport).Clone()
	logs = &bytes.Buffer{}
	srv = httptest.NewServer(NewAPI(cfg, slog.New(slog.NewJSONHandler(logs, nil)), "test", tr, water.NewClient()))
	t.Cleanup(srv.Close)
	return srv, got, logs
}

func do(t *testing.T, method, url, auth, body string) (*http.Response, string) {
	t.Helper()
	req, _ := http.NewRequest(method, url, strings.NewReader(body))
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	req.Header.Set("Cookie", "session=client")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	return resp, string(b)
}

func TestForwardsWithUpstreamKey(t *testing.T) {
	srv, got, logs := setup(t, nil)
	p := srv.URL
	resp, body := do(t, "POST", p+"/svc/chat/completions?model=x", clientAuth, `{"a":1}`)
	if resp.StatusCode != 200 || body != "upstream ok" {
		t.Fatalf("got %d %q", resp.StatusCode, body)
	}
	if got.path != "/v1/chat/completions" || got.query != "model=x" || got.body != `{"a":1}` || got.method != "POST" {
		t.Errorf("upstream saw %+v", got)
	}
	if got.auth != "Bearer upstream-secret" {
		t.Errorf("upstream auth = %q, want injected key", got.auth)
	}
	if got.cookie != "" {
		t.Errorf("client cookie leaked upstream: %q", got.cookie)
	}
	if resp.Header.Get("Set-Cookie") != "" {
		t.Errorf("upstream Set-Cookie reached client")
	}
	srv.Close() // waits for the access log line
	for _, secret := range []string{"client-secret", "upstream-secret", "model=x"} {
		if strings.Contains(logs.String(), secret) {
			t.Errorf("logs contain %q: %s", secret, logs)
		}
	}
}

func TestCustomKeyHeader(t *testing.T) {
	srv, got, _ := setup(t, func(c *config.Config) {
		c.Routes[0].Header, c.Routes[0].Value = "x-api-key", "k"
	})
	p := srv.URL
	do(t, "GET", p+"/svc/x", clientAuth, "")
	if got.auth != "" {
		t.Errorf("client Authorization forwarded: %q", got.auth)
	}
}

func TestRejections(t *testing.T) {
	srv, got, _ := setup(t, nil)
	p := srv.URL
	for name, tc := range map[string]struct {
		method, path, auth, body string
		want                     int
	}{
		"unknown route":   {"GET", "/other/x", clientAuth, "", 404},
		"absolute target": {"GET", "/https://evil.example/x", clientAuth, "", 404},
		"body too large":  {"POST", "/svc/x", clientAuth, strings.Repeat("a", 2048), 413},
	} {
		t.Run(name, func(t *testing.T) {
			*got = seen{}
			resp, _ := do(t, tc.method, p+tc.path, tc.auth, tc.body)
			if resp.StatusCode != tc.want {
				t.Errorf("got %d, want %d", resp.StatusCode, tc.want)
			}
			if got.path != "" {
				t.Errorf("request reached upstream: %+v", got)
			}
		})
	}
}

func TestBodyTooLargeChunked(t *testing.T) {
	srv, _, _ := setup(t, nil)
	p := srv.URL
	req, _ := http.NewRequest("POST", p+"/svc/x", io.MultiReader(strings.NewReader(strings.Repeat("a", 4096))))
	req.ContentLength = -1 // chunked: limit enforced while streaming
	req.Header.Set("Authorization", clientAuth)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 413 {
		t.Errorf("got %d, want 413", resp.StatusCode)
	}
}

func TestStatus(t *testing.T) {
	srv, _, _ := setup(t, nil)
	p := srv.URL
	if resp, body := do(t, "GET", p+"/status", "", ""); resp.StatusCode != 200 || !strings.Contains(body, `"status":"OK"`) {
		t.Errorf("got %d %q", resp.StatusCode, body)
	}
}

func TestUpstreamDown(t *testing.T) {
	srv, _, _ := setup(t, func(c *config.Config) {
		c.Routes[0].Target, _ = url.Parse("https://127.0.0.1:1")
	})
	p := srv.URL
	if resp, _ := do(t, "GET", p+"/svc/x", clientAuth, ""); resp.StatusCode != 502 {
		t.Errorf("got %d, want 502", resp.StatusCode)
	}
}

func TestWaterRoutes(t *testing.T) {
	srv, _, _ := setup(t, nil)
	for name, tc := range map[string]struct {
		method, path, body string
		want               int
		wantBody           string
	}{
		"noaa missing id": {"GET", "/proxy/noaa", "", 400, `{"error":"Station ID is required"}`},
		"aw missing id":   {"POST", "/proxy/aw", `{}`, 400, `{"error":"gaugeSourceIdentifier is required"}`},
		"cdec bad json":   {"POST", "/proxy/cdec", `nope`, 400, `{"error":"station is required"}`},
		"wrong method":    {"GET", "/proxy/cdec", "", 405, ""},
	} {
		t.Run(name, func(t *testing.T) {
			resp, body := do(t, tc.method, srv.URL+tc.path, "", tc.body)
			if resp.StatusCode != tc.want || (tc.wantBody != "" && strings.TrimSpace(body) != tc.wantBody) {
				t.Errorf("got %d %s, want %d %s", resp.StatusCode, body, tc.want, tc.wantBody)
			}
		})
	}
}

func TestWaterRouteReachesUpstream(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Date,Flow,Trend\n05/06/2026 08 AM,395,\n"))
	}))
	t.Cleanup(upstream.Close)
	wc := water.NewClient()
	wc.NOAAURL = upstream.URL
	srv := httptest.NewServer(NewAPI(&config.Config{MaxBodyBytes: 1024}, slog.New(slog.DiscardHandler), "test", http.DefaultTransport, wc))
	t.Cleanup(srv.Close)

	resp, body := do(t, "GET", srv.URL+"/proxy/noaa?id=FOCC1", "", "")
	if resp.StatusCode != 200 || strings.TrimSpace(body) != `[{"dateTime":"2026-05-06T15:00:00.000Z","flow":395,"trend":""}]` {
		t.Errorf("got %d %s", resp.StatusCode, body)
	}
}

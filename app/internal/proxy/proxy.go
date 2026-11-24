// Package proxy forwards requests to one configured upstream.
package proxy

import (
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"

	"github.com/leogoesger/yakkkkk-api/internal/config"
)

// Client headers never forwarded upstream.
var stripRequest = []string{"Authorization", "Proxy-Authorization", "Cookie"}

// New returns a reverse proxy for route, mounted at /{route.Name}/. The
// upstream host is fixed by configuration; clients only choose the path.
func New(route config.Route, transport http.RoundTripper, log *slog.Logger) http.Handler {
	prefix := "/" + route.Name
	return &httputil.ReverseProxy{
		Transport:     transport,
		FlushInterval: -1, // stream responses (e.g. server-sent events) as they arrive
		Rewrite: func(r *httputil.ProxyRequest) {
			r.Out.URL.Path = strings.TrimPrefix(r.In.URL.Path, prefix)
			r.Out.URL.RawPath = ""
			r.SetURL(route.Target)
			for _, h := range stripRequest {
				r.Out.Header.Del(h)
			}
			if route.Value != "" {
				r.Out.Header.Set(route.Header, route.Value)
			}
		},
		ModifyResponse: func(resp *http.Response) error {
			resp.Header.Del("Set-Cookie")
			return nil
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			var tooBig *http.MaxBytesError
			if errors.As(err, &tooBig) {
				http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
				return
			}
			log.Error("upstream error", "route", route.Name, "err", err)
			http.Error(w, "upstream unavailable", http.StatusBadGateway)
		},
	}
}

// Transport is the upstream HTTP client transport. It never retries
// non-idempotent requests and never uses an environment proxy.
func Transport(responseTimeout time.Duration) *http.Transport {
	return &http.Transport{
		DialContext:           (&net.Dialer{Timeout: 10 * time.Second}).DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: responseTimeout,
		IdleConnTimeout:       90 * time.Second,
		MaxIdleConnsPerHost:   16,
		ForceAttemptHTTP2:     true,
	}
}

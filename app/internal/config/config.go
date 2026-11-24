// Package config loads the proxy configuration from environment variables.
package config

import (
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Route struct {
	Name   string   // path prefix: /{Name}/...
	Target *url.URL // upstream base URL
	Header string   // header the upstream key is sent in
	Value  string   // header value, e.g. "Bearer sk-..."
}

type Config struct {
	Addr            string
	Routes          []Route
	MaxBodyBytes    int64
	UpstreamTimeout time.Duration
}

var routeName = regexp.MustCompile(`^[a-z0-9-]+$`)

// Load reads:
//
//	ROUTES                    optional comma-separated name=https://upstream pairs
//	UPSTREAM_<NAME>_KEY       upstream API key (NAME upper-cased, "-" becomes "_")
//	UPSTREAM_<NAME>_HEADER    header for the key; default Authorization, sent as "Bearer <key>"
//	LISTEN_ADDR, MAX_BODY_BYTES, UPSTREAM_TIMEOUT
func Load(getenv func(string) string) (*Config, error) {
	c := &Config{Addr: or(getenv("LISTEN_ADDR"), ":8080")}
	var err error
	if c.MaxBodyBytes, err = strconv.ParseInt(or(getenv("MAX_BODY_BYTES"), "10485760"), 10, 64); err != nil {
		return nil, fmt.Errorf("MAX_BODY_BYTES: %w", err)
	}
	if c.UpstreamTimeout, err = time.ParseDuration(or(getenv("UPSTREAM_TIMEOUT"), "60s")); err != nil {
		return nil, fmt.Errorf("UPSTREAM_TIMEOUT: %w", err)
	}

	seen := map[string]bool{"status": true, "version": true, "proxy": true} // reserved for built-in routes
	for _, pair := range split(getenv("ROUTES")) {
		name, raw, ok := strings.Cut(pair, "=")
		if !ok || !routeName.MatchString(name) {
			return nil, fmt.Errorf("ROUTES: invalid entry %q, want name=https://host", pair)
		}
		if seen[name] {
			return nil, fmt.Errorf("ROUTES: duplicate or reserved name %q", name)
		}
		seen[name] = true
		u, err := url.Parse(raw)
		if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" {
			return nil, fmt.Errorf("ROUTES: %s must be a plain https URL", name)
		}
		env := "UPSTREAM_" + strings.ToUpper(strings.ReplaceAll(name, "-", "_"))
		r := Route{Name: name, Target: u, Header: or(getenv(env+"_HEADER"), "Authorization")}
		if key := getenv(env + "_KEY"); key != "" {
			r.Value = key
			if strings.EqualFold(r.Header, "Authorization") {
				r.Value = "Bearer " + key
			}
		}
		c.Routes = append(c.Routes, r)
	}
	return c, nil
}

// FromEnv loads the configuration from the process environment.
func FromEnv() (*Config, error) { return Load(os.Getenv) }

func or(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func split(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

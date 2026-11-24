package config

import (
	"strings"
	"testing"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func base() map[string]string {
	return map[string]string{
		"ROUTES": "openai=https://api.openai.com, anthropic=https://api.anthropic.com/v1",
	}
}

func TestLoadUpstreamKeys(t *testing.T) {
	m := base()
	m["UPSTREAM_OPENAI_KEY"] = "sk-1"
	m["UPSTREAM_ANTHROPIC_KEY"] = "sk-2"
	m["UPSTREAM_ANTHROPIC_HEADER"] = "x-api-key"
	c, err := Load(env(m))
	if err != nil {
		t.Fatal(err)
	}
	if r := c.Routes[0]; r.Header != "Authorization" || r.Value != "Bearer sk-1" {
		t.Errorf("openai: got %s: %s", r.Header, r.Value)
	}
	if r := c.Routes[1]; r.Header != "x-api-key" || r.Value != "sk-2" || r.Target.Path != "/v1" {
		t.Errorf("anthropic: got %s: %s %s", r.Header, r.Value, r.Target)
	}
}

func TestLoadRejects(t *testing.T) {
	for name, tc := range map[string]struct{ key, value, want string }{
		"plain http":     {"ROUTES", "a=http://example.com", "https"},
		"userinfo":       {"ROUTES", "a=https://u:p@example.com", "https"},
		"no host":        {"ROUTES", "a=https:///x", "https"},
		"reserved name":  {"ROUTES", "status=https://example.com", "reserved"},
		"duplicate":      {"ROUTES", "a=https://x.com,a=https://y.com", "duplicate"},
		"bad name":       {"ROUTES", "A/b=https://x.com", "invalid"},
		"bad body limit": {"MAX_BODY_BYTES", "ten", "MAX_BODY_BYTES"},
	} {
		t.Run(name, func(t *testing.T) {
			m := base()
			m[tc.key] = tc.value
			_, err := Load(env(m))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got %v, want error containing %q", err, tc.want)
			}
		})
	}
}

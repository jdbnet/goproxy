package proxyconfig

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateOK(t *testing.T) {
	cfg := &Config{
		Frontends: []Frontend{
			{ID: "http", Bind: "0.0.0.0:80"},
			{ID: "https", Bind: "0.0.0.0:443", TLS: &FrontendTLS{Enabled: true}},
		},
		ACLs: []ACL{
			{ID: "app", Frontend: "https", Match: Match{Host: "app.example.com"}, Mode: "terminate", Backend: "app-pool", Certificate: "example"},
			{ID: "mail", Frontend: "https", Match: Match{Host: "mail.example.com"}, Mode: "passthrough", Backend: "mail-pool"},
		},
		Backends: []Backend{
			{ID: "app-pool", Algorithm: "round_robin", Servers: []BackendServer{{URL: "http://10.0.0.1:8080"}}},
			{ID: "mail-pool", Algorithm: "round_robin", Servers: []BackendServer{{Address: "10.0.0.2:443"}}},
		},
		Certificates: []Certificate{
			{ID: "example", Domains: []string{"app.example.com"}, Challenge: "http-01"},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestRedirectOnlyACL(t *testing.T) {
	cfg := &Config{
		Frontends: []Frontend{{ID: "https", Bind: "0.0.0.0:443", TLS: &FrontendTLS{Enabled: true}}},
		ACLs: []ACL{{
			Frontend:   "https",
			Match:      Match{Host: "apps.s3.jdbnet.co.uk"},
			Mode:       "terminate",
			Middleware: &Middleware{RedirectURL: "https://apps.jdbnet.co.uk"},
		}},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestACLNeedsBackendOrRedirect(t *testing.T) {
	cfg := &Config{
		Frontends: []Frontend{{ID: "https", Bind: "0.0.0.0:443", TLS: &FrontendTLS{Enabled: true}}},
		ACLs:      []ACL{{Frontend: "https", Match: Match{Host: "x.example.com"}, Mode: "terminate"}},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error")
	}
}

func TestFrontendDefaultRedirect(t *testing.T) {
	cfg := &Config{
		Frontends: []Frontend{{
			ID:      "https",
			Bind:    "0.0.0.0:443",
			TLS:     &FrontendTLS{Enabled: true},
			Default: &FrontendDefault{RedirectURL: "https://www.jdbnet.co.uk"},
		}},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestFrontendDefaultHTTPSRedirect(t *testing.T) {
	cfg := &Config{
		Frontends: []Frontend{{ID: "http", Bind: "0.0.0.0:80", Default: &FrontendDefault{HTTPSRedirect: true}}},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	if !cfg.Frontends[0].HasDefault() {
		t.Fatal("https redirect should count as a default")
	}
}

func TestFrontendDefaultUnknownBackend(t *testing.T) {
	cfg := &Config{
		Frontends: []Frontend{{ID: "https", Bind: "0.0.0.0:443", Default: &FrontendDefault{Backend: "missing"}}},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error")
	}
}

func TestPassthroughRequiresTLSFrontend(t *testing.T) {
	cfg := &Config{
		Frontends: []Frontend{{ID: "http", Bind: "0.0.0.0:80"}},
		ACLs:      []ACL{{ID: "mail", Frontend: "http", Match: Match{Host: "mail.example.com"}, Mode: "passthrough", Backend: "mail-pool"}},
		Backends:  []Backend{{ID: "mail-pool", Servers: []BackendServer{{Address: "10.0.0.2:443"}}}},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error")
	}
}

func TestParseRate(t *testing.T) {
	n, d, err := ParseRate("100/s")
	if err != nil || n != 100 || d.Seconds() != 1 {
		t.Fatalf("got %v %v %v", n, d, err)
	}
}

func TestHostKey(t *testing.T) {
	if HostKey("App.Example.com:443") != "app.example.com" {
		t.Fatal(HostKey("App.Example.com:443"))
	}
}

func TestEnsureIDs(t *testing.T) {
	cfg := &Config{
		Frontends: []Frontend{{Bind: "0.0.0.0:443", TLS: &FrontendTLS{Enabled: true}}},
		Backends:  []Backend{{Servers: []BackendServer{{URL: "http://10.0.0.8:8080"}}}},
		ACLs:      []ACL{{Frontend: "https-443", Match: Match{Host: "App.Example.com"}, Mode: "terminate", Backend: "10-0-0-8"}},
		Certificates: []Certificate{
			{Domains: []string{"*.example.com"}, Challenge: "dns-01", DNSProvider: "cloudflare"},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	if cfg.Frontends[0].ID != "https-443" {
		t.Fatalf("frontend id %s", cfg.Frontends[0].ID)
	}
	if cfg.Backends[0].ID != "10-0-0-8" {
		t.Fatalf("backend id %s", cfg.Backends[0].ID)
	}
	if cfg.ACLs[0].ID != "app-example-com" {
		t.Fatalf("acl id %s", cfg.ACLs[0].ID)
	}
	if cfg.Certificates[0].ID != "wildcard-example-com" {
		t.Fatalf("cert id %s", cfg.Certificates[0].ID)
	}
}

func TestEnsureIDsCollision(t *testing.T) {
	cfg := &Config{
		Frontends: []Frontend{
			{Bind: "0.0.0.0:443", TLS: &FrontendTLS{Enabled: true}},
			{Bind: "127.0.0.1:443", TLS: &FrontendTLS{Enabled: true}},
		},
	}
	cfg.EnsureIDs()
	if cfg.Frontends[0].ID != "https-443" || cfg.Frontends[1].ID != "https-443-2" {
		t.Fatalf("%s %s", cfg.Frontends[0].ID, cfg.Frontends[1].ID)
	}
}

func TestLoadOrCreate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "proxy.yaml")
	cfg, created, err := LoadOrCreate(path)
	if err != nil {
		t.Fatal(err)
	}
	if !created {
		t.Fatal("expected create")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	_, created, err = LoadOrCreate(path)
	if err != nil || created {
		t.Fatalf("second load created=%v err=%v", created, err)
	}
}

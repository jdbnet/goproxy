package proxyconfig

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Frontends     []Frontend    `yaml:"frontends" json:"frontends"`
	ACLs          []ACL         `yaml:"acls" json:"acls"`
	Backends      []Backend     `yaml:"backends" json:"backends"`
	Certificates  []Certificate `yaml:"certificates" json:"certificates"`
	Notifications Notifications `yaml:"notifications" json:"notifications"`
}

type Frontend struct {
	ID      string           `yaml:"id" json:"id"`
	Name    string           `yaml:"name,omitempty" json:"name,omitempty"`
	Bind    string           `yaml:"bind" json:"bind"`
	TLS     *FrontendTLS     `yaml:"tls,omitempty" json:"tls,omitempty"`
	Default *FrontendDefault `yaml:"default,omitempty" json:"default,omitempty"`
}

type FrontendDefault struct {
	Backend          string `yaml:"backend,omitempty" json:"backend,omitempty"`
	RedirectURL      string `yaml:"redirect_url,omitempty" json:"redirect_url,omitempty"`
	RedirectKeepPath *bool  `yaml:"redirect_keep_path,omitempty" json:"redirect_keep_path,omitempty"`
	HTTPSRedirect    bool   `yaml:"https_redirect,omitempty" json:"https_redirect,omitempty"`
}

func (f *Frontend) HasDefault() bool {
	return f != nil && f.Default != nil && (f.Default.RedirectURL != "" || f.Default.Backend != "" || f.Default.HTTPSRedirect)
}

func (d *FrontendDefault) KeepPath() bool {
	if d == nil || d.RedirectKeepPath == nil {
		return true
	}
	return *d.RedirectKeepPath
}

type FrontendTLS struct {
	Enabled    bool     `yaml:"enabled" json:"enabled"`
	MinVersion string   `yaml:"min_version,omitempty" json:"min_version,omitempty"`
	Ciphers    []string `yaml:"ciphers,omitempty" json:"ciphers,omitempty"`
	HSTS       bool     `yaml:"hsts" json:"hsts"`
}

type ACL struct {
	ID          string      `yaml:"id" json:"id"`
	Name        string      `yaml:"name,omitempty" json:"name,omitempty"`
	Frontend    string      `yaml:"frontend" json:"frontend"`
	Match       Match       `yaml:"match" json:"match"`
	Mode        string      `yaml:"mode" json:"mode"`
	Backend     string      `yaml:"backend" json:"backend"`
	Certificate string      `yaml:"certificate,omitempty" json:"certificate,omitempty"`
	Middleware  *Middleware `yaml:"middleware,omitempty" json:"middleware,omitempty"`
	RateLimit   *RateLimit  `yaml:"rate_limit,omitempty" json:"rate_limit,omitempty"`
}

type Match struct {
	Host string `yaml:"host" json:"host"`
}

type Middleware struct {
	HTTPSRedirect         bool              `yaml:"https_redirect,omitempty" json:"https_redirect,omitempty"`
	DomainRedirect        string            `yaml:"domain_redirect,omitempty" json:"domain_redirect,omitempty"`
	RedirectURL           string            `yaml:"redirect_url,omitempty" json:"redirect_url,omitempty"`
	RedirectKeepPath      *bool             `yaml:"redirect_keep_path,omitempty" json:"redirect_keep_path,omitempty"`
	RedirectCode          int               `yaml:"redirect_code,omitempty" json:"redirect_code,omitempty"`
	HeadersAdd            map[string]string `yaml:"headers_add,omitempty" json:"headers_add,omitempty"`
	HeadersRemove         []string          `yaml:"headers_remove,omitempty" json:"headers_remove,omitempty"`
	ResponseHeadersAdd    map[string]string `yaml:"response_headers_add,omitempty" json:"response_headers_add,omitempty"`
	ResponseHeadersRemove []string          `yaml:"response_headers_remove,omitempty" json:"response_headers_remove,omitempty"`
	PathRewrite           *PathRewrite      `yaml:"path_rewrite,omitempty" json:"path_rewrite,omitempty"`
	BasicAuth             *BasicAuth        `yaml:"basic_auth,omitempty" json:"basic_auth,omitempty"`
	IPAllow               []string          `yaml:"ip_allow,omitempty" json:"ip_allow,omitempty"`
	IPDeny                []string          `yaml:"ip_deny,omitempty" json:"ip_deny,omitempty"`
}

func (m *Middleware) HasRedirect() bool {
	if m == nil {
		return false
	}
	return m.RedirectURL != "" || m.DomainRedirect != "" || m.HTTPSRedirect
}

func (m *Middleware) KeepRedirectPath() bool {
	if m == nil || m.RedirectKeepPath == nil {
		return true
	}
	return *m.RedirectKeepPath
}

type PathRewrite struct {
	From string `yaml:"from" json:"from"`
	To   string `yaml:"to" json:"to"`
}

type BasicAuth struct {
	Realm    string          `yaml:"realm,omitempty" json:"realm,omitempty"`
	Paths    []string        `yaml:"paths,omitempty" json:"paths,omitempty"`
	Users    []BasicAuthUser `yaml:"users,omitempty" json:"users,omitempty"`
	Username string          `yaml:"username,omitempty" json:"username,omitempty"`
	Password string          `yaml:"password,omitempty" json:"password,omitempty"`
}

type BasicAuthUser struct {
	Username string `yaml:"username" json:"username"`
	Password string `yaml:"password" json:"password"`
}

func (b *BasicAuth) AllUsers() []BasicAuthUser {
	if b == nil {
		return nil
	}
	users := append([]BasicAuthUser{}, b.Users...)
	if b.Username != "" {
		users = append(users, BasicAuthUser{Username: b.Username, Password: b.Password})
	}
	return users
}

type RateLimit struct {
	PerIP      string `yaml:"per_ip,omitempty" json:"per_ip,omitempty"`
	PerACL     string `yaml:"per_acl,omitempty" json:"per_acl,omitempty"`
	PerBackend string `yaml:"per_backend,omitempty" json:"per_backend,omitempty"`
}

type Backend struct {
	ID        string          `yaml:"id" json:"id"`
	Name      string          `yaml:"name,omitempty" json:"name,omitempty"`
	Algorithm string          `yaml:"algorithm" json:"algorithm"`
	Servers   []BackendServer `yaml:"servers" json:"servers"`
	Health    *HealthCheck    `yaml:"health,omitempty" json:"health,omitempty"`
	RateLimit *RateLimit      `yaml:"rate_limit,omitempty" json:"rate_limit,omitempty"`
}

type BackendServer struct {
	URL     string `yaml:"url,omitempty" json:"url,omitempty"`
	Address string `yaml:"address,omitempty" json:"address,omitempty"`
	Role    string `yaml:"role,omitempty" json:"role,omitempty"`
	Weight  int    `yaml:"weight,omitempty" json:"weight,omitempty"`
}

type HealthCheck struct {
	Type               string `yaml:"type" json:"type"`
	Path               string `yaml:"path,omitempty" json:"path,omitempty"`
	Interval           string `yaml:"interval,omitempty" json:"interval,omitempty"`
	Timeout            string `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	HealthyThreshold   int    `yaml:"healthy_threshold,omitempty" json:"healthy_threshold,omitempty"`
	UnhealthyThreshold int    `yaml:"unhealthy_threshold,omitempty" json:"unhealthy_threshold,omitempty"`
	ExpectStatus       int    `yaml:"expect_status,omitempty" json:"expect_status,omitempty"`
	ExpectBody         string `yaml:"expect_body,omitempty" json:"expect_body,omitempty"`
}

type Certificate struct {
	ID          string   `yaml:"id" json:"id"`
	Name        string   `yaml:"name,omitempty" json:"name,omitempty"`
	Domains     []string `yaml:"domains" json:"domains"`
	Challenge   string   `yaml:"challenge" json:"challenge"`
	DNSProvider string   `yaml:"dns_provider,omitempty" json:"dns_provider,omitempty"`
	CertFile    string   `yaml:"cert_file,omitempty" json:"cert_file,omitempty"`
	KeyFile     string   `yaml:"key_file,omitempty" json:"key_file,omitempty"`
}

type Notifications struct {
	Webhooks []Webhook `yaml:"webhooks" json:"webhooks"`
}

type Webhook struct {
	ID       string   `yaml:"id" json:"id"`
	URL      string   `yaml:"url" json:"url"`
	Format   string   `yaml:"format" json:"format"`
	Triggers []string `yaml:"triggers,omitempty" json:"triggers,omitempty"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read proxy config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse proxy config: %w", err)
	}
	cfg.defaults()
	return &cfg, nil
}

func Empty() *Config {
	return &Config{
		Frontends:    []Frontend{},
		ACLs:         []ACL{},
		Backends:     []Backend{},
		Certificates: []Certificate{},
		Notifications: Notifications{
			Webhooks: []Webhook{},
		},
	}
}

func LoadOrCreate(path string) (*Config, bool, error) {
	cfg, err := Load(path)
	if err == nil {
		return cfg, false, nil
	}
	if !os.IsNotExist(err) && !errors.Is(err, os.ErrNotExist) {
		return nil, false, err
	}
	cfg = Empty()
	if err := Write(path, cfg); err != nil {
		return nil, false, fmt.Errorf("create proxy config: %w", err)
	}
	return cfg, true, nil
}

func Write(path string, cfg *Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (c *Config) Clone() *Config {
	data, err := yaml.Marshal(c)
	if err != nil {
		return &Config{}
	}
	var out Config
	if err := yaml.Unmarshal(data, &out); err != nil {
		return &Config{}
	}
	return &out
}

func (c *Config) defaults() {
	for i := range c.Frontends {
		if c.Frontends[i].Default != nil && !c.Frontends[i].HasDefault() {
			c.Frontends[i].Default = nil
		}
	}
	for i := range c.ACLs {
		if c.ACLs[i].Mode == "" {
			c.ACLs[i].Mode = "terminate"
		}
	}
	for i := range c.Backends {
		if c.Backends[i].Algorithm == "" {
			c.Backends[i].Algorithm = "round_robin"
		}
		for j := range c.Backends[i].Servers {
			if c.Backends[i].Servers[j].Role == "" {
				c.Backends[i].Servers[j].Role = "primary"
			}
			if c.Backends[i].Servers[j].Weight <= 0 {
				c.Backends[i].Servers[j].Weight = 1
			}
		}
		if c.Backends[i].Health != nil {
			h := c.Backends[i].Health
			if h.Interval == "" {
				h.Interval = "5s"
			}
			if h.Timeout == "" {
				h.Timeout = "2s"
			}
			if h.HealthyThreshold <= 0 {
				h.HealthyThreshold = 2
			}
			if h.UnhealthyThreshold <= 0 {
				h.UnhealthyThreshold = 5
			}
			if h.Type == "" {
				h.Type = "tcp"
			}
		}
	}
}

func (c *Config) Validate() error {
	c.defaults()
	c.EnsureIDs()
	frontends := map[string]Frontend{}
	binds := map[string]string{}
	for _, fe := range c.Frontends {
		if fe.ID == "" {
			return fmt.Errorf("frontend missing id")
		}
		if fe.Bind == "" {
			return fmt.Errorf("frontend %s missing bind", fe.ID)
		}
		if _, ok := frontends[fe.ID]; ok {
			return fmt.Errorf("duplicate frontend id %s", fe.ID)
		}
		if other, ok := binds[fe.Bind]; ok {
			return fmt.Errorf("frontends %s and %s share bind %s", fe.ID, other, fe.Bind)
		}
		frontends[fe.ID] = fe
		binds[fe.Bind] = fe.ID
	}

	backends := map[string]Backend{}
	for _, be := range c.Backends {
		if be.ID == "" {
			return fmt.Errorf("backend missing id")
		}
		if _, ok := backends[be.ID]; ok {
			return fmt.Errorf("duplicate backend id %s", be.ID)
		}
		switch be.Algorithm {
		case "round_robin", "least_conn", "weighted", "ip_hash":
		default:
			return fmt.Errorf("backend %s: unknown algorithm %q", be.ID, be.Algorithm)
		}
		if len(be.Servers) == 0 {
			return fmt.Errorf("backend %s has no servers", be.ID)
		}
		for i, s := range be.Servers {
			if s.URL == "" && s.Address == "" {
				return fmt.Errorf("backend %s server %d missing url or address", be.ID, i)
			}
			if s.Role != "primary" && s.Role != "backup" {
				return fmt.Errorf("backend %s server %d: role must be primary or backup", be.ID, i)
			}
		}
		if be.Health != nil {
			switch be.Health.Type {
			case "tcp", "http", "grpc":
			default:
				return fmt.Errorf("backend %s: unknown health type %q", be.ID, be.Health.Type)
			}
			if _, err := time.ParseDuration(be.Health.Interval); err != nil {
				return fmt.Errorf("backend %s: invalid health interval: %w", be.ID, err)
			}
			if _, err := time.ParseDuration(be.Health.Timeout); err != nil {
				return fmt.Errorf("backend %s: invalid health timeout: %w", be.ID, err)
			}
		}
		backends[be.ID] = be
	}

	for _, fe := range c.Frontends {
		if fe.Default == nil {
			continue
		}
		if fe.Default.Backend != "" {
			if _, ok := backends[fe.Default.Backend]; !ok {
				return fmt.Errorf("frontend %s default references unknown backend %s", fe.ID, fe.Default.Backend)
			}
		}
	}

	certs := map[string]Certificate{}
	for _, cert := range c.Certificates {
		if cert.ID == "" {
			return fmt.Errorf("certificate missing id")
		}
		if _, ok := certs[cert.ID]; ok {
			return fmt.Errorf("duplicate certificate id %s", cert.ID)
		}
		if len(cert.Domains) == 0 {
			return fmt.Errorf("certificate %s has no domains", cert.ID)
		}
		switch cert.Challenge {
		case "http-01", "dns-01", "custom":
		default:
			return fmt.Errorf("certificate %s: challenge must be http-01, dns-01, or custom", cert.ID)
		}
		if cert.Challenge == "dns-01" && cert.DNSProvider == "" {
			return fmt.Errorf("certificate %s: dns-01 requires dns_provider", cert.ID)
		}
		if cert.Challenge == "custom" && (cert.CertFile == "" || cert.KeyFile == "") {
			return fmt.Errorf("certificate %s: custom requires cert_file and key_file", cert.ID)
		}
		certs[cert.ID] = cert
	}

	acls := map[string]struct{}{}
	for _, acl := range c.ACLs {
		if acl.ID == "" {
			return fmt.Errorf("acl missing id")
		}
		if _, ok := acls[acl.ID]; ok {
			return fmt.Errorf("duplicate acl id %s", acl.ID)
		}
		acls[acl.ID] = struct{}{}
		if acl.Match.Host == "" {
			return fmt.Errorf("acl %s missing match.host", acl.ID)
		}
		if acl.Mode != "terminate" && acl.Mode != "passthrough" {
			return fmt.Errorf("acl %s: mode must be terminate or passthrough", acl.ID)
		}
		fe, ok := frontends[acl.Frontend]
		if !ok {
			return fmt.Errorf("acl %s references unknown frontend %s", acl.ID, acl.Frontend)
		}
		if acl.Mode == "passthrough" && (fe.TLS == nil || !fe.TLS.Enabled) {
			return fmt.Errorf("acl %s: passthrough requires a TLS-capable frontend", acl.ID)
		}
		if acl.Mode == "terminate" && acl.Certificate != "" {
			if _, ok := certs[acl.Certificate]; !ok {
				return fmt.Errorf("acl %s references unknown certificate %s", acl.ID, acl.Certificate)
			}
		}
		if acl.Backend == "" {
			if !acl.Middleware.HasRedirect() {
				return fmt.Errorf("acl %s needs a backend or a redirect", acl.ID)
			}
		} else {
			be, ok := backends[acl.Backend]
			if !ok {
				return fmt.Errorf("acl %s references unknown backend %s", acl.ID, acl.Backend)
			}
			if acl.Mode == "passthrough" {
				for i, s := range be.Servers {
					if s.Address == "" {
						return fmt.Errorf("acl %s: passthrough backend %s server %d needs address", acl.ID, be.ID, i)
					}
				}
			}
		}
		if acl.RateLimit != nil {
			if err := validateRate(acl.RateLimit.PerIP, acl.ID, "per_ip"); err != nil {
				return err
			}
			if err := validateRate(acl.RateLimit.PerACL, acl.ID, "per_acl"); err != nil {
				return err
			}
			if err := validateRate(acl.RateLimit.PerBackend, acl.ID, "per_backend"); err != nil {
				return err
			}
		}
	}

	for _, wh := range c.Notifications.Webhooks {
		if wh.ID == "" || wh.URL == "" {
			return fmt.Errorf("webhook missing id or url")
		}
		switch wh.Format {
		case "", "generic", "discord", "slack":
		default:
			return fmt.Errorf("webhook %s: format must be generic, discord, or slack", wh.ID)
		}
	}
	return nil
}

func validateRate(spec, id, field string) error {
	if spec == "" {
		return nil
	}
	if _, _, err := ParseRate(spec); err != nil {
		return fmt.Errorf("acl %s: invalid %s %q: %w", id, field, spec, err)
	}
	return nil
}

func ParseRate(spec string) (float64, time.Duration, error) {
	spec = strings.TrimSpace(spec)
	parts := strings.Split(spec, "/")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("want N/s, N/m, or N/h")
	}
	var n float64
	if _, err := fmt.Sscanf(parts[0], "%f", &n); err != nil || n <= 0 {
		return 0, 0, fmt.Errorf("invalid count")
	}
	switch parts[1] {
	case "s":
		return n, time.Second, nil
	case "m":
		return n, time.Minute, nil
	case "h":
		return n, time.Hour, nil
	default:
		return 0, 0, fmt.Errorf("unit must be s, m, or h")
	}
}

func (c *Config) Frontend(id string) *Frontend {
	for i := range c.Frontends {
		if c.Frontends[i].ID == id {
			return &c.Frontends[i]
		}
	}
	return nil
}

func (c *Config) Backend(id string) *Backend {
	for i := range c.Backends {
		if c.Backends[i].ID == id {
			return &c.Backends[i]
		}
	}
	return nil
}

func (c *Config) ACL(id string) *ACL {
	for i := range c.ACLs {
		if c.ACLs[i].ID == id {
			return &c.ACLs[i]
		}
	}
	return nil
}

func (c *Config) Certificate(id string) *Certificate {
	for i := range c.Certificates {
		if c.Certificates[i].ID == id {
			return &c.Certificates[i]
		}
	}
	return nil
}

func HostKey(host string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	if h, _, ok := strings.Cut(host, ":"); ok {
		return h
	}
	return host
}

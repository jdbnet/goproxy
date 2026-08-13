package proxyconfig

import (
	"net"
	"net/url"
	"strings"
	"unicode"
)

func Slug(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	prevDash := false
	for _, r := range s {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			prevDash = false
		default:
			if !prevDash && b.Len() > 0 {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "item"
	}
	return out
}

func UniqueID(used map[string]struct{}, base string) string {
	base = Slug(base)
	if base == "" {
		base = "item"
	}
	id := base
	for n := 2; ; n++ {
		if _, ok := used[id]; !ok {
			if used != nil {
				used[id] = struct{}{}
			}
			return id
		}
		id = base + "-" + itoa(n)
	}
}

func FrontendSeed(fe Frontend) string {
	kind := "http"
	if fe.TLS != nil && fe.TLS.Enabled {
		kind = "https"
	}
	if p := bindPort(fe.Bind); p != "" {
		return kind + "-" + p
	}
	return kind
}

func BackendSeed(be Backend) string {
	if len(be.Servers) == 0 {
		return "backend"
	}
	s := be.Servers[0]
	if s.URL != "" {
		return hostSlug(s.URL)
	}
	return hostSlug(s.Address)
}

func ACLSeed(acl ACL) string {
	if acl.Match.Host != "" {
		return hostSlug(acl.Match.Host)
	}
	return "route"
}

func CertSeed(c Certificate) string {
	if len(c.Domains) == 0 {
		return "cert"
	}
	d := c.Domains[0]
	if strings.HasPrefix(d, "*.") {
		return "wildcard-" + Slug(d[2:])
	}
	return hostSlug(d)
}

func (c *Config) EnsureIDs() {
	usedFE := map[string]struct{}{}
	for i := range c.Frontends {
		if c.Frontends[i].ID == "" {
			c.Frontends[i].ID = UniqueID(usedFE, FrontendSeed(c.Frontends[i]))
		} else {
			usedFE[c.Frontends[i].ID] = struct{}{}
		}
	}
	usedBE := map[string]struct{}{}
	for i := range c.Backends {
		if c.Backends[i].ID == "" {
			c.Backends[i].ID = UniqueID(usedBE, BackendSeed(c.Backends[i]))
		} else {
			usedBE[c.Backends[i].ID] = struct{}{}
		}
	}
	usedCert := map[string]struct{}{}
	for i := range c.Certificates {
		if c.Certificates[i].ID == "" {
			c.Certificates[i].ID = UniqueID(usedCert, CertSeed(c.Certificates[i]))
		} else {
			usedCert[c.Certificates[i].ID] = struct{}{}
		}
	}
	usedACL := map[string]struct{}{}
	for i := range c.ACLs {
		if c.ACLs[i].ID == "" {
			c.ACLs[i].ID = UniqueID(usedACL, ACLSeed(c.ACLs[i]))
		} else {
			usedACL[c.ACLs[i].ID] = struct{}{}
		}
	}
	usedHook := map[string]struct{}{}
	for i := range c.Notifications.Webhooks {
		wh := &c.Notifications.Webhooks[i]
		if wh.ID == "" {
			seed := "webhook"
			if host := hostSlug(wh.URL); host != "" && host != "item" {
				seed = "hook-" + host
			}
			wh.ID = UniqueID(usedHook, seed)
		} else {
			usedHook[wh.ID] = struct{}{}
		}
	}
}

func FrontendIDs(items []Frontend) map[string]struct{} {
	used := map[string]struct{}{}
	for _, fe := range items {
		if fe.ID != "" {
			used[fe.ID] = struct{}{}
		}
	}
	return used
}

func BackendIDs(items []Backend) map[string]struct{} {
	used := map[string]struct{}{}
	for _, be := range items {
		if be.ID != "" {
			used[be.ID] = struct{}{}
		}
	}
	return used
}

func ACLIDs(items []ACL) map[string]struct{} {
	used := map[string]struct{}{}
	for _, a := range items {
		if a.ID != "" {
			used[a.ID] = struct{}{}
		}
	}
	return used
}

func CertIDs(items []Certificate) map[string]struct{} {
	used := map[string]struct{}{}
	for _, c := range items {
		if c.ID != "" {
			used[c.ID] = struct{}{}
		}
	}
	return used
}

func hostSlug(raw string) string {
	raw = strings.TrimSpace(raw)
	if u, err := url.Parse(raw); err == nil && u.Host != "" {
		raw = u.Host
	}
	raw = strings.TrimPrefix(raw, "*.")
	if h, _, err := net.SplitHostPort(raw); err == nil {
		raw = h
	}
	return Slug(raw)
}

func bindPort(bind string) string {
	_, port, err := net.SplitHostPort(bind)
	if err != nil {
		return ""
	}
	return port
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [12]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

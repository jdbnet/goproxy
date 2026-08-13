package mw

import (
	"bufio"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"

	"git.jdbnet.co.uk/jamie/goproxy/internal/proxyconfig"
)

func Apply(next http.Handler, acl *proxyconfig.ACL, frontendTLS bool) http.Handler {
	if acl == nil || acl.Middleware == nil {
		return next
	}
	m := acl.Middleware
	h := next
	h = withHeaders(h, m)
	h = withPathRewrite(h, m)
	h = withRedirects(h, m, frontendTLS)
	h = withBasicAuth(h, m)
	h = withIPFilter(h, m)
	h = withResponseHeaders(h, m)
	return h
}

func withRedirects(next http.Handler, m *proxyconfig.Middleware, frontendTLS bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if m.HTTPSRedirect && !frontendTLS && r.TLS == nil {
			u := *r.URL
			u.Scheme = "https"
			u.Host = r.Host
			http.Redirect(w, r, u.String(), http.StatusMovedPermanently)
			return
		}
		if loc, ok := redirectLocation(m, r); ok {
			code := m.RedirectCode
			if code == 0 {
				code = http.StatusMovedPermanently
			}
			http.Redirect(w, r, loc, code)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func redirectLocation(m *proxyconfig.Middleware, r *http.Request) (string, bool) {
	if m.RedirectURL != "" {
		return JoinRedirect(m.RedirectURL, r, m.KeepRedirectPath()), true
	}
	if m.DomainRedirect != "" {
		u := *r.URL
		if r.TLS != nil {
			u.Scheme = "https"
		} else {
			u.Scheme = "http"
		}
		u.Host = m.DomainRedirect
		return u.String(), true
	}
	return "", false
}

func JoinRedirect(base string, r *http.Request, keepPath bool) string {
	if !keepPath {
		return base
	}
	u, err := url.Parse(base)
	if err != nil || u.Scheme == "" || u.Host == "" {
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		return scheme + "://" + strings.TrimRight(base, "/") + r.URL.RequestURI()
	}
	u.Path = r.URL.Path
	u.RawQuery = r.URL.RawQuery
	u.Fragment = ""
	return u.String()
}

func withHeaders(next http.Handler, m *proxyconfig.Middleware) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, name := range m.HeadersRemove {
			r.Header.Del(name)
		}
		for k, v := range m.HeadersAdd {
			r.Header.Set(k, v)
		}
		next.ServeHTTP(w, r)
	})
}

func withResponseHeaders(next http.Handler, m *proxyconfig.Middleware) http.Handler {
	if len(m.ResponseHeadersAdd) == 0 && len(m.ResponseHeadersRemove) == 0 {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(&headerRewriter{
			ResponseWriter: w,
			add:            m.ResponseHeadersAdd,
			remove:         m.ResponseHeadersRemove,
		}, r)
	})
}

type headerRewriter struct {
	http.ResponseWriter
	add    map[string]string
	remove []string
	wrote  bool
}

func (h *headerRewriter) apply() {
	if h.wrote {
		return
	}
	h.wrote = true
	for _, name := range h.remove {
		h.Header().Del(name)
	}
	for k, v := range h.add {
		h.Header().Set(k, v)
	}
}

func (h *headerRewriter) WriteHeader(code int) {
	h.apply()
	h.ResponseWriter.WriteHeader(code)
}

func (h *headerRewriter) Write(p []byte) (int, error) {
	h.apply()
	return h.ResponseWriter.Write(p)
}

func (h *headerRewriter) Flush() {
	h.apply()
	if f, ok := h.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (h *headerRewriter) Unwrap() http.ResponseWriter {
	return h.ResponseWriter
}

func (h *headerRewriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h.apply()
	hj, ok := h.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("hijack not supported")
	}
	return hj.Hijack()
}

func withPathRewrite(next http.Handler, m *proxyconfig.Middleware) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if m.PathRewrite != nil && m.PathRewrite.From != "" {
			if strings.HasPrefix(r.URL.Path, m.PathRewrite.From) {
				r.URL.Path = m.PathRewrite.To + strings.TrimPrefix(r.URL.Path, m.PathRewrite.From)
				if r.URL.RawPath != "" {
					r.URL.RawPath = ""
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}

func withBasicAuth(next http.Handler, m *proxyconfig.Middleware) http.Handler {
	if m.BasicAuth == nil {
		return next
	}
	users := m.BasicAuth.AllUsers()
	if len(users) == 0 {
		return next
	}
	realm := m.BasicAuth.Realm
	if realm == "" {
		realm = "goproxy"
	}
	wanted := make([][]byte, 0, len(users))
	for _, u := range users {
		wanted = append(wanted, []byte("Basic "+base64.StdEncoding.EncodeToString([]byte(u.Username+":"+u.Password))))
	}
	paths := m.BasicAuth.Paths
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !pathNeedsAuth(r.URL.Path, paths) {
			next.ServeHTTP(w, r)
			return
		}
		got := []byte(r.Header.Get("Authorization"))
		ok := false
		for _, want := range wanted {
			if subtle.ConstantTimeCompare(sha256sum(got), sha256sum(want)) == 1 {
				ok = true
				break
			}
		}
		if !ok {
			w.Header().Set("WWW-Authenticate", `Basic realm="`+realm+`"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func pathNeedsAuth(path string, prefixes []string) bool {
	if len(prefixes) == 0 {
		return true
	}
	for _, p := range prefixes {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if path == p || strings.HasPrefix(path, strings.TrimRight(p, "/")+"/") || path == strings.TrimRight(p, "/") {
			return true
		}
	}
	return false
}

func sha256sum(b []byte) []byte {
	sum := sha256.Sum256(b)
	return sum[:]
}

func withIPFilter(next http.Handler, m *proxyconfig.Middleware) http.Handler {
	if len(m.IPAllow) == 0 && len(m.IPDeny) == 0 {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		if len(m.IPDeny) > 0 && containsIP(m.IPDeny, ip) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if len(m.IPAllow) > 0 && !containsIP(m.IPAllow, ip) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func containsIP(list []string, ip string) bool {
	parsed := net.ParseIP(ip)
	for _, item := range list {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if strings.Contains(item, "/") {
			_, n, err := net.ParseCIDR(item)
			if err == nil && parsed != nil && n.Contains(parsed) {
				return true
			}
			continue
		}
		if item == ip {
			return true
		}
	}
	return false
}

func SetHSTS(w http.ResponseWriter) {
	w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
}

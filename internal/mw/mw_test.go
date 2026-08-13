package mw

import (
	"bufio"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"git.jdbnet.co.uk/jamie/goproxy/internal/proxyconfig"
)

func TestPathRewrite(t *testing.T) {
	acl := &proxyconfig.ACL{Middleware: &proxyconfig.Middleware{PathRewrite: &proxyconfig.PathRewrite{From: "/old", To: "/new"}}}
	h := Apply(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/new/x" {
			t.Fatalf("path %s", r.URL.Path)
		}
	}), acl, false)
	req := httptest.NewRequest(http.MethodGet, "/old/x", nil)
	h.ServeHTTP(httptest.NewRecorder(), req)
}

func TestRedirectURLKeepsPath(t *testing.T) {
	acl := &proxyconfig.ACL{Middleware: &proxyconfig.Middleware{RedirectURL: "https://apps.jdbnet.co.uk"}}
	h := Apply(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should redirect")
	}), acl, true)
	req := httptest.NewRequest(http.MethodGet, "/goproxy-amd64?x=1", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusMovedPermanently {
		t.Fatalf("code %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "https://apps.jdbnet.co.uk/goproxy-amd64?x=1" {
		t.Fatalf("location %s", loc)
	}
}

func TestRedirectURLDropsPath(t *testing.T) {
	keep := false
	acl := &proxyconfig.ACL{Middleware: &proxyconfig.Middleware{RedirectURL: "https://www.yourbalancedmind.co.uk", RedirectKeepPath: &keep}}
	h := Apply(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("should redirect")
	}), acl, true)
	req := httptest.NewRequest(http.MethodGet, "/old", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if loc := rec.Header().Get("Location"); loc != "https://www.yourbalancedmind.co.uk" {
		t.Fatalf("location %s", loc)
	}
}

func TestBasicAuthUsersAndPath(t *testing.T) {
	acl := &proxyconfig.ACL{Middleware: &proxyconfig.Middleware{BasicAuth: &proxyconfig.BasicAuth{
		Realm: "zot",
		Paths: []string{"/v2"},
		Users: []proxyconfig.BasicAuthUser{{Username: "jamie", Password: "secret"}},
	}}}
	called := false
	h := Apply(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}), acl, true)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if !called || rec.Code != http.StatusOK {
		t.Fatalf("open path should pass code=%d called=%v", rec.Code, called)
	}

	called = false
	req = httptest.NewRequest(http.MethodGet, "/v2/", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("protected path code %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/v2/", nil)
	req.SetBasicAuth("jamie", "secret")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("authed code %d", rec.Code)
	}
}

func TestIPAllow(t *testing.T) {
	acl := &proxyconfig.ACL{Middleware: &proxyconfig.Middleware{IPAllow: []string{"5.133.42.60", "92.238.108.209"}}}
	h := Apply(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), acl, true)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "1.2.3.4:9"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("denied code %d", rec.Code)
	}
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "5.133.42.60:9"
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("allowed code %d", rec.Code)
	}
}

type hijackRecorder struct {
	http.ResponseWriter
}

func (hijackRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return nil, nil, nil
}

func TestResponseHeadersPreserveHijack(t *testing.T) {
	acl := &proxyconfig.ACL{Middleware: &proxyconfig.Middleware{
		ResponseHeadersRemove: []string{"X-Frame-Options"},
	}}
	var inner http.ResponseWriter
	h := Apply(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		inner = w
	}), acl, true)
	h.ServeHTTP(hijackRecorder{httptest.NewRecorder()}, httptest.NewRequest(http.MethodGet, "/", nil))
	if _, ok := inner.(http.Hijacker); !ok {
		t.Fatal("response wrapper must keep http.Hijacker for websockets")
	}
}

func TestResponseHeaders(t *testing.T) {
	acl := &proxyconfig.ACL{Middleware: &proxyconfig.Middleware{
		ResponseHeadersRemove: []string{"X-Frame-Options"},
		ResponseHeadersAdd:    map[string]string{"Content-Security-Policy": "frame-ancestors 'self' https://dashboard.example.com"},
	}}
	h := Apply(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Other", "keep")
		w.WriteHeader(http.StatusOK)
	}), acl, true)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Header().Get("X-Frame-Options") != "" {
		t.Fatalf("X-Frame-Options still set")
	}
	if rec.Header().Get("X-Other") != "keep" {
		t.Fatalf("lost unrelated header")
	}
	if rec.Header().Get("Content-Security-Policy") != "frame-ancestors 'self' https://dashboard.example.com" {
		t.Fatalf("csp %q", rec.Header().Get("Content-Security-Policy"))
	}
}

func TestIPDeny(t *testing.T) {
	acl := &proxyconfig.ACL{Middleware: &proxyconfig.Middleware{IPDeny: []string{"10.0.0.1"}}}
	h := Apply(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should deny")
	}), acl, false)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:9"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("code %d", rec.Code)
	}
}

package api

import (
	"encoding/json"
	"io"
	"io/fs"
	"net/http"
	"strings"

	"git.jdbnet.co.uk/jamie/goproxy/docs"
	"git.jdbnet.co.uk/jamie/goproxy/internal/audit"
	"git.jdbnet.co.uk/jamie/goproxy/internal/auth"
	"git.jdbnet.co.uk/jamie/goproxy/internal/backup"
	"git.jdbnet.co.uk/jamie/goproxy/internal/config"
	"git.jdbnet.co.uk/jamie/goproxy/internal/gitsync"
	"git.jdbnet.co.uk/jamie/goproxy/internal/metrics"
	"git.jdbnet.co.uk/jamie/goproxy/internal/notify"
	"git.jdbnet.co.uk/jamie/goproxy/internal/proxy"
	"git.jdbnet.co.uk/jamie/goproxy/internal/proxyconfig"
	"git.jdbnet.co.uk/jamie/goproxy/internal/tlsx"
	"git.jdbnet.co.uk/jamie/goproxy/ui"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Server struct {
	app      *config.Config
	auth     *auth.Service
	audit    *audit.Log
	cfg      *ConfigService
	engine   *proxy.Engine
	certs    *tlsx.Store
	metrics  *metrics.Metrics
	backup   *backup.Manager
	git      *gitsync.Sync
	notify   *notify.Notifier
	hasUsers bool
}

func New(app *config.Config, authSvc *auth.Service, al *audit.Log, cfg *ConfigService, engine *proxy.Engine, certs *tlsx.Store, m *metrics.Metrics, b *backup.Manager, git *gitsync.Sync, n *notify.Notifier, hasUsers bool) *Server {
	return &Server{
		app: app, auth: authSvc, audit: al, cfg: cfg, engine: engine, certs: certs,
		metrics: m, backup: b, git: git, notify: n, hasUsers: hasUsers,
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/health", s.health)
	mux.HandleFunc("GET /api/v1/auth/status", s.authStatus)
	mux.HandleFunc("POST /api/v1/auth/login", s.login)
	mux.Handle("GET /metrics", promhttp.Handler())
	mux.HandleFunc("GET /api-docs", s.swaggerUI)
	mux.HandleFunc("GET /api-docs/openapi.yaml", s.openapi)

	prot := http.NewServeMux()
	prot.HandleFunc("POST /api/v1/auth/logout", s.logout)
	prot.HandleFunc("GET /api/v1/auth/me", s.me)
	prot.HandleFunc("GET /api/v1/frontends", auth.Require("frontends:read", s.listFrontends))
	prot.HandleFunc("POST /api/v1/frontends", auth.Require("frontends:write", s.createFrontend))
	prot.HandleFunc("PUT /api/v1/frontends/{id}", auth.Require("frontends:write", s.updateFrontend))
	prot.HandleFunc("DELETE /api/v1/frontends/{id}", auth.Require("frontends:write", s.deleteFrontend))
	prot.HandleFunc("GET /api/v1/acls", auth.Require("acls:read", s.listACLs))
	prot.HandleFunc("POST /api/v1/acls", auth.Require("acls:write", s.createACL))
	prot.HandleFunc("PUT /api/v1/acls/{id}", auth.Require("acls:write", s.updateACL))
	prot.HandleFunc("DELETE /api/v1/acls/{id}", auth.Require("acls:write", s.deleteACL))
	prot.HandleFunc("GET /api/v1/backends/status", auth.Require("backends:read", s.backendStatus))
	prot.HandleFunc("GET /api/v1/backends", auth.Require("backends:read", s.listBackends))
	prot.HandleFunc("POST /api/v1/backends", auth.Require("backends:write", s.createBackend))
	prot.HandleFunc("PUT /api/v1/backends/{id}", auth.Require("backends:write", s.updateBackend))
	prot.HandleFunc("DELETE /api/v1/backends/{id}", auth.Require("backends:write", s.deleteBackend))
	prot.HandleFunc("GET /api/v1/dns-providers", auth.Require("certs:read", s.listDNSProviders))
	prot.HandleFunc("GET /api/v1/certificates", auth.Require("certs:read", s.listCerts))
	prot.HandleFunc("POST /api/v1/certificates", auth.Require("certs:write", s.createCert))
	prot.HandleFunc("PUT /api/v1/certificates/{id}", auth.Require("certs:write", s.updateCert))
	prot.HandleFunc("DELETE /api/v1/certificates/{id}", auth.Require("certs:write", s.deleteCert))
	prot.HandleFunc("POST /api/v1/certificates/{id}/renew", auth.Require("certs:write", s.renewCert))
	prot.HandleFunc("POST /api/v1/certificates/{id}/upload", auth.Require("certs:write", s.uploadCert))
	prot.HandleFunc("GET /api/v1/certificates/{id}/job", auth.Require("certs:read", s.certJob))
	prot.HandleFunc("GET /api/v1/users", auth.Require("users:read", s.listUsers))
	prot.HandleFunc("POST /api/v1/users", auth.Require("users:write", s.createUser))
	prot.HandleFunc("PUT /api/v1/users/{id}", auth.Require("users:write", s.updateUser))
	prot.HandleFunc("DELETE /api/v1/users/{id}", auth.Require("users:write", s.deleteUser))
	prot.HandleFunc("GET /api/v1/apikeys", auth.Require("apikeys:read", s.listKeys))
	prot.HandleFunc("POST /api/v1/apikeys", auth.Require("apikeys:write", s.createKey))
	prot.HandleFunc("DELETE /api/v1/apikeys/{id}", auth.Require("apikeys:write", s.deleteKey))
	prot.HandleFunc("GET /api/v1/audit", auth.Require("audit:read", s.listAudit))
	prot.HandleFunc("GET /api/v1/stats", auth.Require("stats:read", s.stats))
	prot.HandleFunc("GET /api/v1/stats/history", auth.Require("stats:read", s.statsHistory))
	prot.HandleFunc("GET /api/v1/config", auth.Require("settings:read", s.getConfig))
	prot.HandleFunc("PUT /api/v1/config", auth.Require("settings:write", s.putConfig))
	prot.HandleFunc("POST /api/v1/config/validate", auth.Require("settings:write", s.validateConfig))
	prot.HandleFunc("POST /api/v1/config/sync", auth.Require("settings:write", s.syncConfig))
	prot.HandleFunc("POST /api/v1/backup", auth.Require("settings:write", s.runBackup))
	prot.HandleFunc("GET /api/v1/settings", auth.Require("settings:read", s.getSettings))
	prot.HandleFunc("GET /api/v1/notifications", auth.Require("settings:read", s.listHooks))
	prot.HandleFunc("PUT /api/v1/notifications", auth.Require("settings:write", s.putHooks))
	prot.HandleFunc("POST /api/v1/notifications/test", auth.Require("settings:write", s.testHook))

	mux.Handle("/api/v1/", s.protect(prot))
	mux.Handle("/", spaHandler())
	return mux
}

func (s *Server) protect(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.hasUsers {
			actor := &auth.Actor{Type: "system", ID: "bootstrap", Scopes: authAll()}
			h.ServeHTTP(w, r.WithContext(auth.WithActor(r.Context(), actor)))
			return
		}
		s.auth.Middleware(h).ServeHTTP(w, r)
	})
}

func authAll() []string {
	return []string{
		"frontends:read", "frontends:write", "backends:read", "backends:write",
		"acls:read", "acls:write", "certs:read", "certs:write",
		"users:read", "users:write", "apikeys:read", "apikeys:write",
		"audit:read", "audit:write", "stats:read", "stats:write",
		"settings:read", "settings:write",
	}
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) authStatus(w http.ResponseWriter, r *http.Request) {
	authenticated := false
	if c, err := r.Cookie("goproxy_session"); err == nil && c.Value != "" {
		if _, err := s.auth.SessionUser(c.Value); err == nil {
			authenticated = true
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"auth_required": s.hasUsers,
		"authenticated": authenticated,
	})
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	u, err := s.auth.Authenticate(req.Username, req.Password)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	token, err := s.auth.CreateSession(u.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	auth.SetSessionCookie(w, token)
	_ = s.audit.Record(audit.ActorUser, u.Username, "auth.login", "session", nil, nil)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "user": u})
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie("goproxy_session"); err == nil {
		_ = s.auth.DeleteSession(c.Value)
	}
	auth.ClearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	a := auth.ActorFrom(r.Context())
	writeJSON(w, http.StatusOK, a)
}

func (s *Server) swaggerUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(`<!DOCTYPE html>
<html><head><title>GoProxy API</title>
<link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5.9.0/swagger-ui.css">
</head><body>
<div id="swagger-ui"></div>
<script src="https://unpkg.com/swagger-ui-dist@5.9.0/swagger-ui-bundle.js"></script>
<script>SwaggerUIBundle({url:'/api-docs/openapi.yaml',dom_id:'#swagger-ui'})</script>
</body></html>`))
}

func (s *Server) openapi(w http.ResponseWriter, r *http.Request) {
	b, err := docs.FS.ReadFile("openapi.yaml")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/yaml")
	_, _ = w.Write(b)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return false
	}
	return true
}

func spaHandler() http.Handler {
	sub, err := fs.Sub(ui.FS, "dist")
	if err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "ui not built", http.StatusNotFound)
		})
	}
	fileServer := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			f, err := sub.Open(strings.TrimPrefix(r.URL.Path, "/"))
			if err == nil {
				_ = f.Close()
				fileServer.ServeHTTP(w, r)
				return
			}
		}
		index, err := sub.Open("index.html")
		if err != nil {
			http.Error(w, "ui not built", http.StatusNotFound)
			return
		}
		defer index.Close()
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.Copy(w, index)
	})
}

func (s *Server) actor(r *http.Request) *auth.Actor {
	return auth.ActorFrom(r.Context())
}

func removeByID[T any](items []T, id string, get func(T) string) []T {
	var out []T
	for _, it := range items {
		if get(it) != id {
			out = append(out, it)
		}
	}
	return out
}

func replaceByID[T any](items []T, id string, next T, get func(T) string) []T {
	found := false
	for i, it := range items {
		if get(it) == id {
			items[i] = next
			found = true
			break
		}
	}
	if !found {
		items = append(items, next)
	}
	return items
}

func findID[T any](items []T, id string, get func(T) string) (T, bool) {
	var zero T
	for _, it := range items {
		if get(it) == id {
			return it, true
		}
	}
	return zero, false
}

var _ = proxyconfig.Config{}

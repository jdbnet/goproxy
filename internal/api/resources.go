package api

import (
	"net/http"
	"strconv"
	"strings"

	"git.jdbnet.co.uk/jamie/goproxy/internal/auth"
	"git.jdbnet.co.uk/jamie/goproxy/internal/config"
	"git.jdbnet.co.uk/jamie/goproxy/internal/notify"
	"git.jdbnet.co.uk/jamie/goproxy/internal/proxyconfig"
	"git.jdbnet.co.uk/jamie/goproxy/internal/tlsx"
)

func (s *Server) listFrontends(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.cfg.Current().Frontends)
}

func (s *Server) createFrontend(w http.ResponseWriter, r *http.Request) {
	var fe proxyconfig.Frontend
	if !decodeJSON(w, r, &fe) {
		return
	}
	if fe.ID == "" {
		fe.ID = proxyconfig.UniqueID(proxyconfig.FrontendIDs(s.cfg.Current().Frontends), proxyconfig.FrontendSeed(fe))
	}
	err := s.cfg.Mutate(s.actor(r), "frontend.create", "frontends/"+fe.ID, nil, fe, func(c *proxyconfig.Config) error {
		c.Frontends = append(c.Frontends, fe)
		return nil
	})
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, fe)
}

func (s *Server) updateFrontend(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var fe proxyconfig.Frontend
	if !decodeJSON(w, r, &fe) {
		return
	}
	fe.ID = id
	prev, _ := findID(s.cfg.Current().Frontends, id, func(f proxyconfig.Frontend) string { return f.ID })
	err := s.cfg.Mutate(s.actor(r), "frontend.update", "frontends/"+id, prev, fe, func(c *proxyconfig.Config) error {
		c.Frontends = replaceByID(c.Frontends, id, fe, func(f proxyconfig.Frontend) string { return f.ID })
		return nil
	})
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, fe)
}

func (s *Server) deleteFrontend(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	prev, _ := findID(s.cfg.Current().Frontends, id, func(f proxyconfig.Frontend) string { return f.ID })
	err := s.cfg.Mutate(s.actor(r), "frontend.delete", "frontends/"+id, prev, nil, func(c *proxyconfig.Config) error {
		c.Frontends = removeByID(c.Frontends, id, func(f proxyconfig.Frontend) string { return f.ID })
		return nil
	})
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listACLs(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.cfg.Current().ACLs)
}

func (s *Server) createACL(w http.ResponseWriter, r *http.Request) {
	var acl proxyconfig.ACL
	if !decodeJSON(w, r, &acl) {
		return
	}
	if acl.ID == "" {
		acl.ID = proxyconfig.UniqueID(proxyconfig.ACLIDs(s.cfg.Current().ACLs), proxyconfig.ACLSeed(acl))
	}
	err := s.cfg.Mutate(s.actor(r), "acl.create", "acls/"+acl.ID, nil, acl, func(c *proxyconfig.Config) error {
		c.ACLs = append(c.ACLs, acl)
		return nil
	})
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, acl)
}

func (s *Server) updateACL(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var acl proxyconfig.ACL
	if !decodeJSON(w, r, &acl) {
		return
	}
	acl.ID = id
	prev, _ := findID(s.cfg.Current().ACLs, id, func(a proxyconfig.ACL) string { return a.ID })
	err := s.cfg.Mutate(s.actor(r), "acl.update", "acls/"+id, prev, acl, func(c *proxyconfig.Config) error {
		c.ACLs = replaceByID(c.ACLs, id, acl, func(a proxyconfig.ACL) string { return a.ID })
		return nil
	})
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, acl)
}

func (s *Server) deleteACL(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	prev, _ := findID(s.cfg.Current().ACLs, id, func(a proxyconfig.ACL) string { return a.ID })
	err := s.cfg.Mutate(s.actor(r), "acl.delete", "acls/"+id, prev, nil, func(c *proxyconfig.Config) error {
		c.ACLs = removeByID(c.ACLs, id, func(a proxyconfig.ACL) string { return a.ID })
		return nil
	})
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listBackends(w http.ResponseWriter, r *http.Request) {
	s.engine.RefreshMetrics()
	writeJSON(w, http.StatusOK, s.cfg.Current().Backends)
}

func (s *Server) backendStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.engine.BackendStatus())
}

func (s *Server) createBackend(w http.ResponseWriter, r *http.Request) {
	var be proxyconfig.Backend
	if !decodeJSON(w, r, &be) {
		return
	}
	if be.ID == "" {
		be.ID = proxyconfig.UniqueID(proxyconfig.BackendIDs(s.cfg.Current().Backends), proxyconfig.BackendSeed(be))
	}
	err := s.cfg.Mutate(s.actor(r), "backend.create", "backends/"+be.ID, nil, be, func(c *proxyconfig.Config) error {
		c.Backends = append(c.Backends, be)
		return nil
	})
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, be)
}

func (s *Server) updateBackend(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var be proxyconfig.Backend
	if !decodeJSON(w, r, &be) {
		return
	}
	be.ID = id
	prev, _ := findID(s.cfg.Current().Backends, id, func(b proxyconfig.Backend) string { return b.ID })
	err := s.cfg.Mutate(s.actor(r), "backend.update", "backends/"+id, prev, be, func(c *proxyconfig.Config) error {
		c.Backends = replaceByID(c.Backends, id, be, func(b proxyconfig.Backend) string { return b.ID })
		return nil
	})
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, be)
}

func (s *Server) deleteBackend(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	prev, ok := findID(s.cfg.Current().Backends, id, func(b proxyconfig.Backend) string { return b.ID })
	if !ok {
		writeErr(w, http.StatusNotFound, "backend not found")
		return
	}
	err := s.cfg.Mutate(s.actor(r), "backend.delete", "backends/"+id, prev, nil, func(c *proxyconfig.Config) error {
		c.Backends = removeByID(c.Backends, id, func(b proxyconfig.Backend) string { return b.ID })
		c.RemoveBackendReferences(id)
		return nil
	})
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type certWriteRequest struct {
	proxyconfig.Certificate
	DNSCredentials map[string]string `json:"dns_credentials,omitempty"`
	CertPEM        string            `json:"cert_pem,omitempty"`
	KeyPEM         string            `json:"key_pem,omitempty"`
}

type certView struct {
	proxyconfig.Certificate
	DNSCredentialsSet bool     `json:"dns_credentials_set"`
	DNSCredentialKeys []string `json:"dns_credential_keys,omitempty"`
	ExpiresAt         string   `json:"expires_at,omitempty"`
	DaysLeft          *int     `json:"days_left,omitempty"`
}

func (s *Server) listDNSProviders(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, tlsx.DNSProviders())
}

func (s *Server) certView(c proxyconfig.Certificate) certView {
	keys := s.certs.DNSCredentialKeys(c.ID)
	exp := s.certs.Expiry(c.ID, c.CertFile)
	return certView{
		Certificate:       c,
		DNSCredentialsSet: len(keys) > 0,
		DNSCredentialKeys: keys,
		ExpiresAt:         exp.ExpiresAt,
		DaysLeft:          exp.DaysLeft,
	}
}

func (s *Server) listCerts(w http.ResponseWriter, r *http.Request) {
	var out []certView
	for _, c := range s.cfg.Current().Certificates {
		out = append(out, s.certView(c))
	}
	if out == nil {
		out = []certView{}
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) applyCertExtras(id string, req certWriteRequest, c *proxyconfig.Certificate) error {
	if len(req.DNSCredentials) > 0 {
		if err := s.certs.SetDNSCredentials(id, req.DNSCredentials); err != nil {
			return err
		}
	}
	if req.CertPEM != "" && req.KeyPEM != "" {
		if err := s.certs.InstallCustom(id, []byte(req.CertPEM), []byte(req.KeyPEM)); err != nil {
			return err
		}
		c.Challenge = "custom"
		c.CertFile = s.certs.CertPath(id)
		c.KeyFile = s.certs.KeyPath(id)
	}
	return nil
}

func (s *Server) certJob(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	job := s.certs.Job(id)
	if job == nil {
		writeJSON(w, http.StatusOK, map[string]any{"id": id, "status": "idle", "lines": []any{}})
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (s *Server) createCert(w http.ResponseWriter, r *http.Request) {
	var req certWriteRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	c := req.Certificate
	if c.ID == "" {
		c.ID = proxyconfig.UniqueID(proxyconfig.CertIDs(s.cfg.Current().Certificates), proxyconfig.CertSeed(c))
	}
	s.certs.BeginJob(c.ID, "issue")
	s.certs.JobLog(c.ID, "saving certificate config")
	if err := s.applyCertExtras(c.ID, req, &c); err != nil {
		s.certs.FinishJob(c.ID, err)
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	err := s.cfg.Mutate(s.actor(r), "cert.create", "certs/"+c.ID, nil, c, func(cfg *proxyconfig.Config) error {
		cfg.Certificates = append(cfg.Certificates, c)
		return nil
	})
	if err != nil {
		s.certs.FinishJob(c.ID, err)
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	s.finishIfStillRunning(c.ID)
	writeJSON(w, http.StatusCreated, s.certView(c))
}

func (s *Server) updateCert(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req certWriteRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	c := req.Certificate
	c.ID = id
	s.certs.BeginJob(id, "issue")
	s.certs.JobLog(id, "updating certificate config")
	if err := s.applyCertExtras(id, req, &c); err != nil {
		s.certs.FinishJob(id, err)
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	prev, _ := findID(s.cfg.Current().Certificates, id, func(x proxyconfig.Certificate) string { return x.ID })
	if c.CertFile == "" {
		c.CertFile = prev.CertFile
	}
	if c.KeyFile == "" {
		c.KeyFile = prev.KeyFile
	}
	err := s.cfg.Mutate(s.actor(r), "cert.update", "certs/"+id, prev, c, func(cfg *proxyconfig.Config) error {
		cfg.Certificates = replaceByID(cfg.Certificates, id, c, func(x proxyconfig.Certificate) string { return x.ID })
		return nil
	})
	if err != nil {
		s.certs.FinishJob(id, err)
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	s.finishIfStillRunning(id)
	writeJSON(w, http.StatusOK, s.certView(c))
}

func (s *Server) finishIfStillRunning(id string) {
	if job := s.certs.Job(id); job != nil && job.Status == "running" {
		s.certs.JobLog(id, "config applied")
		s.certs.FinishJob(id, nil)
	}
}

func (s *Server) deleteCert(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	prev, _ := findID(s.cfg.Current().Certificates, id, func(x proxyconfig.Certificate) string { return x.ID })
	err := s.cfg.Mutate(s.actor(r), "cert.delete", "certs/"+id, prev, nil, func(cfg *proxyconfig.Config) error {
		cfg.Certificates = removeByID(cfg.Certificates, id, func(x proxyconfig.Certificate) string { return x.ID })
		return nil
	})
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	_ = s.certs.DeleteDNSCredentials(id)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) uploadCert(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		CertPEM string `json:"cert_pem"`
		KeyPEM  string `json:"key_pem"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.CertPEM == "" || req.KeyPEM == "" {
		writeErr(w, http.StatusBadRequest, "cert_pem and key_pem required")
		return
	}
	if err := s.certs.InstallCustom(id, []byte(req.CertPEM), []byte(req.KeyPEM)); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	err := s.cfg.Mutate(s.actor(r), "cert.upload", "certs/"+id, nil, id, func(cfg *proxyconfig.Config) error {
		for i := range cfg.Certificates {
			if cfg.Certificates[i].ID == id {
				cfg.Certificates[i].Challenge = "custom"
				cfg.Certificates[i].CertFile = s.certs.CertPath(id)
				cfg.Certificates[i].KeyFile = s.certs.KeyPath(id)
			}
		}
		return nil
	})
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "uploaded"})
}

func (s *Server) renewCert(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.certs.BeginJob(id, "renew")
	s.certs.JobLog(id, "starting renewal")
	if err := s.certs.Renew(id); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "renewed"})
}

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.auth.ListUsers()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, users)
}

func (s *Server) createUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string    `json:"username"`
		Password string    `json:"password"`
		Role     auth.Role `json:"role"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	u, err := s.auth.CreateUser(req.Username, req.Password, req.Role)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	s.hasUsers = true
	a := s.actor(r)
	_ = s.audit.Record(a.Type, a.ID, "user.create", "users/"+req.Username, nil, u)
	writeJSON(w, http.StatusCreated, u)
}

func (s *Server) updateUser(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	var req struct {
		Password string    `json:"password"`
		Role     auth.Role `json:"role"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.auth.UpdateUser(id, req.Password, req.Role); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	u, _ := s.auth.GetUser(id)
	writeJSON(w, http.StatusOK, u)
}

func (s *Server) deleteUser(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err := s.auth.DeleteUser(id); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listKeys(w http.ResponseWriter, r *http.Request) {
	keys, err := s.auth.ListAPIKeys()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, keys)
}

func (s *Server) createKey(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name   string   `json:"name"`
		Scopes []string `json:"scopes"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	var createdBy *int64
	if a := s.actor(r); a != nil && a.User != nil {
		createdBy = &a.User.ID
	}
	plain, key, err := s.auth.CreateAPIKey(req.Name, req.Scopes, createdBy)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	a := s.actor(r)
	_ = s.audit.Record(a.Type, a.ID, "apikey.create", "apikeys/"+req.Name, nil, key)
	writeJSON(w, http.StatusCreated, map[string]any{"key": key, "token": plain})
}

func (s *Server) updateKey(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	var req struct {
		Scopes []string `json:"scopes"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	key, err := s.auth.UpdateAPIKeyScopes(id, req.Scopes)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	a := s.actor(r)
	_ = s.audit.Record(a.Type, a.ID, "apikey.update", "apikeys/"+key.Name, nil, key)
	writeJSON(w, http.StatusOK, key)
}

func (s *Server) deleteKey(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err := s.auth.DeleteAPIKey(id); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listAudit(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	entries, err := s.audit.List(limit, offset)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, entries)
}

func (s *Server) stats(w http.ResponseWriter, r *http.Request) {
	s.engine.RefreshMetrics()
	writeJSON(w, http.StatusOK, s.metrics.Live())
}

func (s *Server) statsHistory(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"history":    s.metrics.History(),
		"limitation": "In-memory ring buffer (1h). Resets on process restart. Scrape /metrics for durable history.",
	})
}

func (s *Server) getConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.cfg.Current())
}

func (s *Server) putConfig(w http.ResponseWriter, r *http.Request) {
	var next proxyconfig.Config
	if !decodeJSON(w, r, &next) {
		return
	}
	if err := s.cfg.Replace(s.actor(r), &next); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, next)
}

func (s *Server) validateConfig(w http.ResponseWriter, r *http.Request) {
	var next proxyconfig.Config
	if !decodeJSON(w, r, &next) {
		return
	}
	if err := next.Validate(); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) syncConfig(w http.ResponseWriter, r *http.Request) {
	if s.git == nil || !s.git.Enabled() {
		writeErr(w, http.StatusBadRequest, "git sync is disabled")
		return
	}
	sha, err := s.git.Pull()
	if err != nil {
		if s.notify != nil {
			s.notify.Send(notify.Event{Type: "git.conflict", Title: "Git pull failed", Body: err.Error(), Severity: "error"})
		}
		writeErr(w, http.StatusConflict, err.Error())
		return
	}
	if err := s.cfg.ApplyFromDisk("system:gitsync", "gitsync", sha); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"sha": sha})
}

func (s *Server) runBackup(w http.ResponseWriter, r *http.Request) {
	if err := s.backup.Run(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) getSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.settingsView())
}

func (s *Server) settingsView() map[string]any {
	return map[string]any{
		"listen":         s.app.Listen,
		"log_level":      s.app.LogLevel,
		"log_requests":   s.app.LogRequests,
		"data_dir":       s.app.DataDir,
		"proxy_config":   s.app.ProxyConfig,
		"acme_email":     s.app.ACMEEmail,
		"acme_directory": s.app.ACMEDirectory,
		"git_enabled":    s.app.Git.Enabled,
		"git_url":        s.app.Git.URL,
		"git_branch":     s.app.Git.Branch,
		"git": map[string]any{
			"enabled":   s.app.Git.Enabled,
			"url":       s.app.Git.URL,
			"branch":    s.app.Git.Branch,
			"auth":      s.app.Git.Auth,
			"key_path":  s.app.Git.KeyPath,
			"token_set": s.app.Git.Token != "",
		},
		"backup":  s.app.Backup,
		"tls":     s.app.TLS,
		"version": s.version,
	}
}

func (s *Server) putSettings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ACMEEmail   string `json:"acme_email"`
		LogRequests *bool  `json:"log_requests"`
		Git         struct {
			Enabled bool   `json:"enabled"`
			URL     string `json:"url"`
			Branch  string `json:"branch"`
			Auth    string `json:"auth"`
			KeyPath string `json:"key_path"`
			Token   string `json:"token"`
		} `json:"git"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	s.settingsMu.Lock()
	defer s.settingsMu.Unlock()

	before := s.settingsView()
	email := strings.TrimSpace(req.ACMEEmail)
	if err := config.ValidateACMEEmail(email); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	git := s.app.Git
	git.Enabled = req.Git.Enabled
	git.URL = strings.TrimSpace(req.Git.URL)
	git.Branch = strings.TrimSpace(req.Git.Branch)
	if git.Branch == "" {
		git.Branch = "main"
	}
	git.Auth = strings.TrimSpace(req.Git.Auth)
	if git.Auth == "" {
		git.Auth = "ssh"
	}
	git.KeyPath = strings.TrimSpace(req.Git.KeyPath)
	if token := strings.TrimSpace(req.Git.Token); token != "" {
		git.Token = token
	}
	if err := config.ValidateGit(git); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if s.git != nil {
		if err := s.git.Reconfigure(git); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	s.app.ACMEEmail = email
	if req.LogRequests != nil {
		s.app.LogRequests = *req.LogRequests
	}
	s.app.Git = git
	if s.configPath == "" {
		writeErr(w, http.StatusInternalServerError, "config path not set")
		return
	}
	if err := config.Save(s.configPath, s.app); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if a := s.actor(r); s.audit != nil {
		_ = s.audit.Record(a.Type, a.ID, "settings.update", "config.yaml", before, s.settingsView())
	}
	writeJSON(w, http.StatusOK, s.settingsView())
}

func (s *Server) listHooks(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.cfg.Current().Notifications.Webhooks)
}

func (s *Server) putHooks(w http.ResponseWriter, r *http.Request) {
	var hooks []proxyconfig.Webhook
	if !decodeJSON(w, r, &hooks) {
		return
	}
	err := s.cfg.Mutate(s.actor(r), "notifications.update", "notifications", s.cfg.Current().Notifications, hooks, func(c *proxyconfig.Config) error {
		c.Notifications.Webhooks = hooks
		return nil
	})
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, hooks)
}

func (s *Server) testHook(w http.ResponseWriter, r *http.Request) {
	var hook proxyconfig.Webhook
	if !decodeJSON(w, r, &hook) {
		return
	}
	if hook.URL == "" {
		writeErr(w, http.StatusBadRequest, "url is required")
		return
	}
	if s.notify == nil {
		writeErr(w, http.StatusServiceUnavailable, "notifications unavailable")
		return
	}
	if err := s.notify.Test(hook); err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

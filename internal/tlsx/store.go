package tlsx

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log/slog"
	"math/big"
	mrand "math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"git.jdbnet.co.uk/jamie/goproxy/internal/config"
	"git.jdbnet.co.uk/jamie/goproxy/internal/metrics"
	"git.jdbnet.co.uk/jamie/goproxy/internal/notify"
	"git.jdbnet.co.uk/jamie/goproxy/internal/proxyconfig"
	"github.com/go-acme/lego/v4/certcrypto"
	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/lego"
	legoLog "github.com/go-acme/lego/v4/log"
	"github.com/go-acme/lego/v4/registration"
)

type Store struct {
	dir     string
	app     *config.Config
	notify  *notify.Notifier
	metrics *metrics.Metrics
	creds   *credStore
	jobs    *jobBook
	envMu   sync.Mutex

	mu     sync.RWMutex
	certs  map[string]*tls.Certificate
	bySNI  map[string]string
	http01 sync.Map
	cfg    *proxyconfig.Config
}

func NewStore(app *config.Config, n *notify.Notifier, m *metrics.Metrics) *Store {
	cs := newCredStore(app.DataDir)
	_ = cs.load()
	return &Store{
		dir:     filepath.Join(app.DataDir, "certs"),
		app:     app,
		notify:  n,
		metrics: m,
		creds:   cs,
		jobs:    newJobBook(),
		certs:   map[string]*tls.Certificate{},
		bySNI:   map[string]string{},
	}
}

func (s *Store) BeginJob(id, action string) {
	s.jobs.Begin(id, action)
}

func (s *Store) JobLog(id, msg string) {
	s.jobs.Append(id, msg)
	slog.Info("cert job", "id", id, "msg", msg)
}

func (s *Store) FinishJob(id string, err error) {
	s.jobs.Finish(id, err)
}

func (s *Store) Job(id string) *Job {
	return s.jobs.Get(id)
}

func (s *Store) SetDNSCredentials(id string, creds map[string]string) error {
	return s.creds.Set(id, creds)
}

func (s *Store) DNSCredentialKeys(id string) []string {
	return s.creds.Keys(id)
}

func (s *Store) DeleteDNSCredentials(id string) error {
	return s.creds.Delete(id)
}

func (s *Store) Reload(cfg *proxyconfig.Config) error {
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return err
	}
	next := map[string]*tls.Certificate{}
	bySNI := map[string]string{}
	for _, c := range cfg.Certificates {
		cert, err := s.loadOrIssue(c)
		if err != nil {
			slog.Error("certificate load", "id", c.ID, "err", err)
			continue
		}
		next[c.ID] = cert
		for _, d := range c.Domains {
			bySNI[d] = c.ID
		}
		if s.metrics != nil && len(cert.Certificate) > 0 {
			if parsed, err := x509.ParseCertificate(cert.Certificate[0]); err == nil {
				s.metrics.SetCertExpiry(c.ID, time.Until(parsed.NotAfter).Seconds())
			}
		}
	}
	s.mu.Lock()
	s.certs = next
	s.bySNI = bySNI
	s.cfg = cfg
	s.mu.Unlock()
	return nil
}

func (s *Store) loadOrIssue(c proxyconfig.Certificate) (*tls.Certificate, error) {
	if c.Challenge == "custom" {
		cert, err := tls.LoadX509KeyPair(c.CertFile, c.KeyFile)
		return &cert, err
	}
	crtPath := filepath.Join(s.dir, c.ID+".crt")
	keyPath := filepath.Join(s.dir, c.ID+".key")
	if cert, err := tls.LoadX509KeyPair(crtPath, keyPath); err == nil {
		return &cert, nil
	}
	return s.issue(c)
}

func (s *Store) CertPath(id string) string { return filepath.Join(s.dir, id+".crt") }
func (s *Store) KeyPath(id string) string  { return filepath.Join(s.dir, id+".key") }

func (s *Store) InstallCustom(id string, certPEM, keyPEM []byte) error {
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return err
	}
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return err
	}
	if err := os.WriteFile(s.CertPath(id), certPEM, 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(s.KeyPath(id), keyPEM, 0o600); err != nil {
		return err
	}
	s.mu.Lock()
	s.certs[id] = &cert
	s.mu.Unlock()
	return nil
}

func (s *Store) GetCertificate(chi *tls.ClientHelloInfo) (*tls.Certificate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if id, ok := s.bySNI[chi.ServerName]; ok {
		if c, ok := s.certs[id]; ok {
			return c, nil
		}
	}
	if c, ok := s.certs[chi.ServerName]; ok {
		return c, nil
	}
	for name, id := range s.bySNI {
		if matchWildcard(name, chi.ServerName) {
			if c, ok := s.certs[id]; ok {
				return c, nil
			}
		}
	}
	return selfSigned(chi.ServerName)
}

func matchWildcard(pattern, name string) bool {
	if len(pattern) < 2 || pattern[0] != '*' || pattern[1] != '.' {
		return false
	}
	suffix := pattern[1:]
	if len(name) <= len(suffix) {
		return false
	}
	return name[len(name)-len(suffix):] == suffix && name != suffix[1:]
}

func (s *Store) ServeHTTP01(w http.ResponseWriter, r *http.Request) {
	token := filepath.Base(r.URL.Path)
	if v, ok := s.http01.Load(token); ok {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(v.(string)))
		return
	}
	http.NotFound(w, r)
}

func (s *Store) Present(_ string, token, keyAuth string) error {
	s.http01.Store(token, keyAuth)
	return nil
}

func (s *Store) CleanUp(_ string, token, _ string) error {
	s.http01.Delete(token)
	return nil
}

type acmeUser struct {
	Email        string
	Registration *registration.Resource
	key          crypto.PrivateKey
}

func (u *acmeUser) GetEmail() string                         { return u.Email }
func (u *acmeUser) GetRegistration() *registration.Resource { return u.Registration }
func (u *acmeUser) GetPrivateKey() crypto.PrivateKey        { return u.key }

func (s *Store) issue(c proxyconfig.Certificate) (*tls.Certificate, error) {
	if s.jobs.Get(c.ID) == nil || s.jobs.Get(c.ID).Status != "running" {
		s.jobs.Begin(c.ID, "issue")
	}
	s.JobLog(c.ID, "starting "+c.Challenge+" issuance for "+strings.Join(c.Domains, ", "))
	prevLog := legoLog.Logger
	legoLog.Logger = jobLegoLogger{append: func(msg string) { s.jobs.Append(c.ID, msg) }}
	defer func() { legoLog.Logger = prevLog }()

	if s.app.ACMEEmail == "" {
		err := fmt.Errorf("acme_email is required to issue certificates")
		s.jobs.Finish(c.ID, err)
		return nil, err
	}
	s.JobLog(c.ID, "registering ACME account")
	user, client, err := s.acmeClient()
	if err != nil {
		s.jobs.Finish(c.ID, err)
		return nil, err
	}
	_ = user
	obtain := func() (*certificate.Resource, error) {
		s.JobLog(c.ID, "requesting certificate from ACME (this can take a few minutes)")
		return client.Certificate.Obtain(certificate.ObtainRequest{Domains: c.Domains, Bundle: true})
	}
	var res *certificate.Resource
	switch c.Challenge {
	case "http-01":
		s.JobLog(c.ID, "using HTTP-01; waiting for Let's Encrypt to hit /.well-known/acme-challenge/")
		if err := client.Challenge.SetHTTP01Provider(s); err != nil {
			s.jobs.Finish(c.ID, err)
			return nil, err
		}
		res, err = obtain()
	case "dns-01":
		stored := s.creds.Get(c.ID)
		name := providerName(c.DNSProvider, stored)
		s.JobLog(c.ID, "using DNS-01 provider "+name)
		err = s.withEnv(stored, func() error {
			p, err := newDNSProvider(name)
			if err != nil {
				return err
			}
			if err := client.Challenge.SetDNS01Provider(p); err != nil {
				return err
			}
			s.JobLog(c.ID, "waiting for DNS propagation")
			res, err = obtain()
			return err
		})
	default:
		err = fmt.Errorf("unsupported challenge %s", c.Challenge)
	}
	if err != nil {
		s.jobs.Finish(c.ID, err)
		return nil, err
	}
	s.JobLog(c.ID, "writing certificate to disk")
	if err := s.writeCert(c.ID, res); err != nil {
		s.jobs.Finish(c.ID, err)
		return nil, err
	}
	cert, err := tls.X509KeyPair(res.Certificate, res.PrivateKey)
	if err != nil {
		s.jobs.Finish(c.ID, err)
		return nil, err
	}
	s.jobs.Finish(c.ID, nil)
	return &cert, nil
}

func (s *Store) Renew(id string) error {
	s.mu.RLock()
	cfg := s.cfg
	s.mu.RUnlock()
	if cfg == nil {
		return fmt.Errorf("no proxy config loaded")
	}
	c := cfg.Certificate(id)
	if c == nil {
		return fmt.Errorf("unknown certificate %s", id)
	}
	if c.Challenge == "custom" {
		return fmt.Errorf("custom certificates are not renewed")
	}
	cert, err := s.issue(*c)
	if err != nil {
		if s.notify != nil {
			s.notify.Send(notify.Event{Type: "cert.renew.failure", Title: "Certificate renewal failed", Body: id + ": " + err.Error(), Severity: "error"})
		}
		return err
	}
	s.mu.Lock()
	s.certs[id] = cert
	s.mu.Unlock()
	if s.notify != nil {
		s.notify.Send(notify.Event{Type: "cert.renew.success", Title: "Certificate renewed", Body: id, Severity: "info"})
	}
	return nil
}

func (s *Store) withEnv(creds map[string]string, fn func() error) error {
	env := envMapForIssue(creds)
	s.envMu.Lock()
	defer s.envMu.Unlock()
	prev := map[string]string{}
	unset := map[string]bool{}
	for k, v := range env {
		if cur, ok := os.LookupEnv(k); ok {
			prev[k] = cur
		} else {
			unset[k] = true
		}
		if err := os.Setenv(k, v); err != nil {
			return err
		}
	}
	defer func() {
		for k, v := range prev {
			_ = os.Setenv(k, v)
		}
		for k := range unset {
			_ = os.Unsetenv(k)
		}
	}()
	return fn()
}

func (s *Store) writeCert(id string, res *certificate.Resource) error {
	if err := os.WriteFile(filepath.Join(s.dir, id+".crt"), res.Certificate, 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.dir, id+".key"), res.PrivateKey, 0o600)
}

func (s *Store) acmeClient() (*acmeUser, *lego.Client, error) {
	acmeDir := filepath.Join(s.app.DataDir, "acme")
	if err := os.MkdirAll(acmeDir, 0o700); err != nil {
		return nil, nil, err
	}
	keyPath := filepath.Join(acmeDir, "account.key")
	regPath := filepath.Join(acmeDir, "account.json")
	var key crypto.PrivateKey
	if raw, err := os.ReadFile(keyPath); err == nil {
		block, _ := pem.Decode(raw)
		if block != nil {
			if k, err := x509.ParseECPrivateKey(block.Bytes); err == nil {
				key = k
			}
		}
	}
	if key == nil {
		k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			return nil, nil, err
		}
		b, err := x509.MarshalECPrivateKey(k)
		if err != nil {
			return nil, nil, err
		}
		if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: b}), 0o600); err != nil {
			return nil, nil, err
		}
		key = k
	}
	user := &acmeUser{Email: s.app.ACMEEmail, key: key}
	if raw, err := os.ReadFile(regPath); err == nil {
		_ = json.Unmarshal(raw, &user.Registration)
	}
	cfg := lego.NewConfig(user)
	cfg.CADirURL = s.app.ACMEDirectory
	cfg.Certificate.KeyType = certcrypto.EC256
	client, err := lego.NewClient(cfg)
	if err != nil {
		return nil, nil, err
	}
	if user.Registration == nil {
		reg, err := client.Registration.Register(registration.RegisterOptions{TermsOfServiceAgreed: true})
		if err != nil {
			return nil, nil, err
		}
		user.Registration = reg
		b, _ := json.Marshal(reg)
		_ = os.WriteFile(regPath, b, 0o600)
	}
	return user, client, nil
}

func (s *Store) StartRenewer(ctx context.Context) {
	check := s.app.TLS.RenewCheck.Duration
	if check <= 0 {
		check = 24 * time.Hour
	}
	jitter := s.app.TLS.Jitter.Duration
	before := s.app.TLS.RenewBefore.Duration
	if before <= 0 {
		before = 30 * 24 * time.Hour
	}
	backoff := s.app.TLS.RetryBackoff.Duration
	if backoff <= 0 {
		backoff = 15 * time.Minute
	}
	maxBackoff := s.app.TLS.RetryMax.Duration
	if maxBackoff <= 0 {
		maxBackoff = 8 * time.Hour
	}

	sleep := func(d time.Duration) bool {
		if jitter > 0 {
			d += time.Duration(mrand.Int63n(int64(jitter)))
		}
		t := time.NewTimer(d)
		defer t.Stop()
		select {
		case <-ctx.Done():
			return false
		case <-t.C:
			return true
		}
	}

	fails := map[string]time.Duration{}
	if !sleep(time.Minute) {
		return
	}
	for {
		s.mu.RLock()
		cfg := s.cfg
		s.mu.RUnlock()
		if cfg != nil {
			for _, c := range cfg.Certificates {
				if c.Challenge == "custom" {
					continue
				}
				need, err := s.needsRenew(c.ID, before)
				if err != nil {
					slog.Error("cert inspect", "id", c.ID, "err", err)
					continue
				}
				if !need {
					delete(fails, c.ID)
					continue
				}
				if err := s.Renew(c.ID); err != nil {
					slog.Error("cert renew failed", "id", c.ID, "err", err)
					wait := fails[c.ID]
					if wait == 0 {
						wait = backoff
					} else {
						wait *= 2
						if wait > maxBackoff {
							wait = maxBackoff
						}
					}
					fails[c.ID] = wait
					continue
				}
				delete(fails, c.ID)
			}
		}
		if !sleep(check) {
			return
		}
	}
}

type ExpiryInfo struct {
	ExpiresAt string `json:"expires_at,omitempty"`
	DaysLeft  *int   `json:"days_left,omitempty"`
}

func (s *Store) Expiry(id, certFile string) ExpiryInfo {
	var der []byte
	s.mu.RLock()
	cert, ok := s.certs[id]
	s.mu.RUnlock()
	if ok && cert != nil && len(cert.Certificate) > 0 {
		der = cert.Certificate[0]
	} else {
		path := certFile
		if path == "" {
			path = s.CertPath(id)
		}
		if raw, err := os.ReadFile(path); err == nil {
			block, _ := pem.Decode(raw)
			if block != nil {
				der = block.Bytes
			}
		}
	}
	if len(der) == 0 {
		return ExpiryInfo{}
	}
	parsed, err := x509.ParseCertificate(der)
	if err != nil {
		return ExpiryInfo{}
	}
	days := int(time.Until(parsed.NotAfter).Hours() / 24)
	return ExpiryInfo{
		ExpiresAt: parsed.NotAfter.UTC().Format(time.RFC3339),
		DaysLeft:  &days,
	}
}

func (s *Store) needsRenew(id string, before time.Duration) (bool, error) {
	s.mu.RLock()
	cert, ok := s.certs[id]
	s.mu.RUnlock()
	if !ok || cert == nil || len(cert.Certificate) == 0 {
		return true, nil
	}
	parsed, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return true, err
	}
	if s.metrics != nil {
		s.metrics.SetCertExpiry(id, time.Until(parsed.NotAfter).Seconds())
	}
	return time.Until(parsed.NotAfter) < before, nil
}

func selfSigned(name string) (*tls.Certificate, error) {
	if name == "" {
		name = "localhost"
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		DNSNames:     []string{name},
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, err
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, err
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	return &cert, err
}

func MinVersion(s string) uint16 {
	switch s {
	case "1.0":
		return tls.VersionTLS10
	case "1.1":
		return tls.VersionTLS11
	case "1.3":
		return tls.VersionTLS13
	default:
		return tls.VersionTLS12
	}
}

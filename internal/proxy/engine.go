package proxy

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"git.jdbnet.co.uk/jamie/goproxy/internal/health"
	"git.jdbnet.co.uk/jamie/goproxy/internal/lb"
	"git.jdbnet.co.uk/jamie/goproxy/internal/metrics"
	"git.jdbnet.co.uk/jamie/goproxy/internal/mw"
	"git.jdbnet.co.uk/jamie/goproxy/internal/notify"
	"git.jdbnet.co.uk/jamie/goproxy/internal/proxyconfig"
	"git.jdbnet.co.uk/jamie/goproxy/internal/ratelimit"
	"git.jdbnet.co.uk/jamie/goproxy/internal/tlsx"
)

type Engine struct {
	pools   *lb.Registry
	health  *health.Checker
	certs   *tlsx.Store
	metrics *metrics.Metrics
	limit   *ratelimit.Limiter
	notify  *notify.Notifier

	cfg atomic.Pointer[proxyconfig.Config]

	mu        sync.Mutex
	frontends map[string]*frontend
	conns     atomic.Int64
	access    *slog.Logger
	transport http.RoundTripper
}

type frontend struct {
	id         string
	bind       string
	tlsEnabled bool
	hsts       bool
	minVersion uint16
	ln         net.Listener
	queue      *connQueue
	http       *http.Server
	cancel     context.CancelFunc
}

type connQueue struct {
	ch     chan net.Conn
	addr   net.Addr
	closed atomic.Bool
}

func newConnQueue(addr net.Addr) *connQueue {
	return &connQueue{ch: make(chan net.Conn, 64), addr: addr}
}

func (q *connQueue) Accept() (net.Conn, error) {
	c, ok := <-q.ch
	if !ok {
		return nil, net.ErrClosed
	}
	return c, nil
}

func (q *connQueue) Close() error {
	if q.closed.CompareAndSwap(false, true) {
		close(q.ch)
	}
	return nil
}

func (q *connQueue) Addr() net.Addr { return q.addr }

func (q *connQueue) push(c net.Conn) {
	if q.closed.Load() {
		c.Close()
		return
	}
	select {
	case q.ch <- c:
	default:
		c.Close()
	}
}

func New(certs *tlsx.Store, pools *lb.Registry, h *health.Checker, m *metrics.Metrics, n *notify.Notifier) *Engine {
	access := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	return &Engine{
		pools:     pools,
		health:    h,
		certs:     certs,
		metrics:   m,
		limit:     ratelimit.New(),
		notify:    n,
		frontends: map[string]*frontend{},
		access:    access,
		transport: backendTransport(),
	}
}

func (e *Engine) Config() *proxyconfig.Config {
	return e.cfg.Load()
}

func (e *Engine) Apply(cfg *proxyconfig.Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	e.pools.Replace(cfg)
	if err := e.certs.Reload(cfg); err != nil {
		return err
	}
	e.health.Reload(cfg)
	e.notify.Reload(cfg)
	e.cfg.Store(cfg)

	e.mu.Lock()
	defer e.mu.Unlock()

	want := map[string]proxyconfig.Frontend{}
	for _, fe := range cfg.Frontends {
		want[fe.ID] = fe
	}
	for id, f := range e.frontends {
		next, ok := want[id]
		if !ok || next.Bind != f.bind || tlsOn(next) != f.tlsEnabled {
			e.stopFrontend(f)
			delete(e.frontends, id)
		}
	}
	for _, fe := range cfg.Frontends {
		if _, ok := e.frontends[fe.ID]; ok {
			f := e.frontends[fe.ID]
			f.hsts = fe.TLS != nil && fe.TLS.HSTS
			if fe.TLS != nil {
				f.minVersion = tlsx.MinVersion(fe.TLS.MinVersion)
			}
			continue
		}
		nf, err := e.startFrontend(fe)
		if err != nil {
			return err
		}
		e.frontends[fe.ID] = nf
	}
	return nil
}

func tlsOn(fe proxyconfig.Frontend) bool {
	return fe.TLS != nil && fe.TLS.Enabled
}

func (e *Engine) startFrontend(fe proxyconfig.Frontend) (*frontend, error) {
	ln, err := net.Listen("tcp", fe.Bind)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	f := &frontend{
		id:         fe.ID,
		bind:       fe.Bind,
		tlsEnabled: tlsOn(fe),
		ln:         ln,
		cancel:     cancel,
	}
	if fe.TLS != nil {
		f.hsts = fe.TLS.HSTS
		f.minVersion = tlsx.MinVersion(fe.TLS.MinVersion)
	}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		e.serveHTTP(f, w, r)
	})
	f.http = &http.Server{Handler: handler, ReadHeaderTimeout: 10 * time.Second}
	if f.tlsEnabled {
		f.queue = newConnQueue(ln.Addr())
		go func() { _ = f.http.Serve(f.queue) }()
		go e.acceptTLS(ctx, f)
	} else {
		go func() { _ = f.http.Serve(ln) }()
	}
	slog.Info("frontend listening", "id", fe.ID, "bind", fe.Bind, "tls", f.tlsEnabled)
	return f, nil
}

func (e *Engine) stopFrontend(f *frontend) {
	f.cancel()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = f.http.Shutdown(ctx)
	if f.queue != nil {
		_ = f.queue.Close()
	}
	if f.ln != nil {
		_ = f.ln.Close()
	}
}

func (e *Engine) Shutdown(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	for id, f := range e.frontends {
		e.stopFrontend(f)
		delete(e.frontends, id)
	}
	e.health.Stop()
	return nil
}

func (e *Engine) acceptTLS(ctx context.Context, f *frontend) {
	for {
		conn, err := f.ln.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				if errors.Is(err, net.ErrClosed) {
					return
				}
				continue
			}
		}
		go e.dispatchTLS(f, conn)
	}
}

type peekedConn struct {
	net.Conn
	r io.Reader
}

func (c *peekedConn) Read(p []byte) (int, error) { return c.r.Read(p) }

func (e *Engine) dispatchTLS(f *frontend, conn net.Conn) {
	e.conns.Add(1)
	e.metrics.SetConns(int(e.conns.Load()))
	defer func() {
		e.conns.Add(-1)
		e.metrics.SetConns(int(e.conns.Load()))
	}()

	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	sni, peeked, err := tlsx.PeekSNI(conn)
	_ = conn.SetReadDeadline(time.Time{})
	if err != nil {
		conn.Close()
		return
	}
	wrapped := &peekedConn{Conn: conn, r: io.MultiReader(bytes.NewReader(peeked), conn)}
	cfg := e.cfg.Load()
	acl := matchACL(cfg, f.id, sni)
	if acl == nil {
		if fe := cfg.Frontend(f.id); !fe.HasDefault() {
			conn.Close()
			return
		}
	} else if acl.Mode == "passthrough" {
		e.passthrough(acl, wrapped)
		return
	}
	tlsCfg := &tls.Config{
		GetCertificate: e.certs.GetCertificate,
		MinVersion:     f.minVersion,
		NextProtos:     []string{"h2", "http/1.1"},
	}
	tlsConn := tls.Server(wrapped, tlsCfg)
	if err := tlsConn.Handshake(); err != nil {
		tlsConn.Close()
		return
	}
	f.queue.push(tlsConn)
}

func (e *Engine) passthrough(acl *proxyconfig.ACL, client net.Conn) {
	defer client.Close()
	pool := e.pools.Pool(acl.Backend)
	srv := pool.Pick(lb.ClientIP(client.RemoteAddr().String()))
	if srv == nil {
		return
	}
	srv.Conns.Add(1)
	defer srv.Conns.Add(-1)
	start := time.Now()
	up, err := net.DialTimeout("tcp", srv.Target(), 5*time.Second)
	dial := time.Since(start)
	if err != nil {
		health.MarkPassiveFailure(srv)
		e.metrics.ObserveRequest(dial, 502, 0, 0, acl.Frontend, acl.ID, acl.Backend, srv.Target())
		return
	}
	defer up.Close()
	errc := make(chan error, 2)
	var inB, outB atomic.Int64
	go func() {
		n, err := io.Copy(up, client)
		inB.Add(n)
		errc <- err
	}()
	go func() {
		n, err := io.Copy(client, up)
		outB.Add(n)
		errc <- err
	}()
	<-errc
	_ = client.Close()
	_ = up.Close()
	<-errc
	e.metrics.ObserveRequest(dial, 200, inB.Load(), outB.Load(), acl.Frontend, acl.ID, acl.Backend, srv.Target())
	e.logAccess(map[string]any{
		"type": "access", "mode": "passthrough", "acl": acl.ID, "sni": acl.Match.Host,
		"backend": srv.Target(), "duration_ms": time.Since(start).Milliseconds(),
	})
}

func (e *Engine) serveHTTP(f *frontend, w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/.well-known/acme-challenge/") {
		e.certs.ServeHTTP01(w, r)
		return
	}
	cfg := e.cfg.Load()
	host := proxyconfig.HostKey(r.Host)
	acl := matchACL(cfg, f.id, host)
	if acl == nil {
		e.serveFrontendDefault(f, w, r)
		return
	}
	if acl.Mode == "passthrough" {
		http.Error(w, "no route", http.StatusNotFound)
		return
	}
	if f.hsts && (f.tlsEnabled || r.TLS != nil) {
		mw.SetHSTS(w)
	}
	ip := lb.ClientIP(r.RemoteAddr)
	if !e.limit.AllowACL(acl, acl.Backend, ip) {
		http.Error(w, "rate limited", http.StatusTooManyRequests)
		return
	}
	if be := cfg.Backend(acl.Backend); be != nil && be.RateLimit != nil {
		if !e.limit.Allow("backend:"+be.ID+":"+ip, be.RateLimit.PerIP) {
			http.Error(w, "rate limited", http.StatusTooManyRequests)
			return
		}
		if !e.limit.Allow("backend:"+be.ID, be.RateLimit.PerBackend) {
			http.Error(w, "rate limited", http.StatusTooManyRequests)
			return
		}
	}
	var handler http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "no backend", http.StatusBadGateway)
	})
	if acl.Backend != "" {
		handler = e.reverseProxy(acl)
	}
	mw.Apply(handler, acl, f.tlsEnabled).ServeHTTP(w, r)
}

func (e *Engine) serveFrontendDefault(f *frontend, w http.ResponseWriter, r *http.Request) {
	cfg := e.cfg.Load()
	fe := cfg.Frontend(f.id)
	if !fe.HasDefault() {
		http.Error(w, "no route", http.StatusNotFound)
		return
	}
	if f.hsts && (f.tlsEnabled || r.TLS != nil) {
		mw.SetHSTS(w)
	}
	d := fe.Default
	if d.HTTPSRedirect {
		http.Redirect(w, r, mw.HTTPSRedirectURL(r), http.StatusMovedPermanently)
		return
	}
	if d.RedirectURL != "" {
		http.Redirect(w, r, mw.JoinRedirect(d.RedirectURL, r, d.KeepPath()), http.StatusMovedPermanently)
		return
	}
	if d.Backend != "" {
		acl := &proxyconfig.ACL{ID: fe.ID + "-default", Frontend: fe.ID, Backend: d.Backend, Mode: "terminate"}
		e.reverseProxy(acl).ServeHTTP(w, r)
		return
	}
	http.Error(w, "no route", http.StatusNotFound)
}

func (e *Engine) reverseProxy(acl *proxyconfig.ACL) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pool := e.pools.Pool(acl.Backend)
		srv := pool.Pick(lb.ClientIP(r.RemoteAddr))
		if srv == nil || srv.URL == nil {
			http.Error(w, "no healthy backend", http.StatusBadGateway)
			return
		}
		srv.Conns.Add(1)
		defer srv.Conns.Add(-1)
		start := time.Now()
		body := &countingBody{ReadCloser: r.Body}
		r.Body = body
		rec := &statusRecorder{ResponseWriter: w, status: 200}
		proxy := &httputil.ReverseProxy{
			Transport: e.transport,
			Rewrite: func(pr *httputil.ProxyRequest) {
				pr.SetURL(srv.URL)
				pr.Out.Host = pr.In.Host
				pr.SetXForwarded()
				pr.Out.Header.Set("X-Forwarded-Port", forwardedPort(pr.In))
			},
			ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
				health.MarkPassiveFailure(srv)
				slog.Warn("proxy error", "acl", acl.ID, "host", r.Host, "path", r.URL.Path, "backend", srv.Target(), "err", err)
				http.Error(w, "bad gateway", http.StatusBadGateway)
			},
			FlushInterval: 100 * time.Millisecond,
		}
		proxy.ServeHTTP(rec, r)
		e.metrics.ObserveRequest(time.Since(start), rec.status, body.n, rec.bytes, acl.Frontend, acl.ID, acl.Backend, srv.Target())
		e.logAccess(map[string]any{
			"type": "access", "mode": "terminate", "acl": acl.ID, "host": r.Host,
			"path": r.URL.Path, "status": rec.status, "backend": srv.Target(),
			"duration_ms": time.Since(start).Milliseconds(),
		})
		if rec.status >= 500 {
			e.maybeErrorRate()
		}
	})
}

func backendTransport() http.RoundTripper {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	return t
}

func forwardedPort(r *http.Request) string {
	if _, p, err := net.SplitHostPort(r.Host); err == nil && p != "" {
		return p
	}
	if r.TLS != nil {
		return "443"
	}
	return "80"
}

func (e *Engine) maybeErrorRate() {
	live := e.metrics.Live()
	if live.Requests > 20 && live.Errors/live.Requests > 0.2 {
		e.notify.Send(notify.Event{
			Type: "errors.high", Title: "High error rate",
			Body: "Recent window has elevated 5xx responses", Severity: "warn",
		})
	}
}

func (e *Engine) logAccess(fields map[string]any) {
	b, _ := json.Marshal(fields)
	var rec map[string]any
	_ = json.Unmarshal(b, &rec)
	attrs := make([]any, 0, len(rec)*2)
	for k, v := range rec {
		attrs = append(attrs, k, v)
	}
	e.access.Info("request", attrs...)
}

func matchACL(cfg *proxyconfig.Config, frontendID, host string) *proxyconfig.ACL {
	if cfg == nil {
		return nil
	}
	host = proxyconfig.HostKey(host)
	var wildcard *proxyconfig.ACL
	for i := range cfg.ACLs {
		acl := &cfg.ACLs[i]
		if acl.Frontend != frontendID {
			continue
		}
		pat := proxyconfig.HostKey(acl.Match.Host)
		if pat == host {
			return acl
		}
		if strings.HasPrefix(pat, "*.") && strings.HasSuffix(host, pat[1:]) {
			wildcard = acl
		}
	}
	return wildcard
}

type countingBody struct {
	io.ReadCloser
	n int64
}

func (c *countingBody) Read(p []byte) (int, error) {
	n, err := c.ReadCloser.Read(p)
	c.n += int64(n)
	return n, err
}

type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int64
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(p []byte) (int, error) {
	n, err := s.ResponseWriter.Write(p)
	s.bytes += int64(n)
	return n, err
}

func (s *statusRecorder) Flush() {
	if f, ok := s.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (s *statusRecorder) Unwrap() http.ResponseWriter {
	return s.ResponseWriter
}

func (s *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := s.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("hijack not supported")
	}
	return h.Hijack()
}

type ServerStatus struct {
	Backend   string  `json:"backend"`
	Name      string  `json:"name,omitempty"`
	Target    string  `json:"target"`
	Role      string  `json:"role"`
	Healthy   bool    `json:"healthy"`
	Conns     int64   `json:"conns"`
	Requests  int64   `json:"requests"`
	BytesIn   int64   `json:"bytes_in"`
	BytesOut  int64   `json:"bytes_out"`
	LatencyMs float64 `json:"latency_ms"`
	ProbeMs   float64 `json:"probe_ms,omitempty"`
}

func (e *Engine) BackendStatus() []ServerStatus {
	names := map[string]string{}
	if cfg := e.Config(); cfg != nil {
		for _, be := range cfg.Backends {
			if be.Name != "" {
				names[be.ID] = be.Name
			}
		}
	}
	var out []ServerStatus
	for _, p := range e.pools.All() {
		for _, s := range p.Servers {
			st := e.metrics.ServerStats(p.ID, s.Target())
			probeMs := float64(s.ProbeLatency()) / float64(time.Millisecond)
			lat := st.LatencyMs
			if lat == 0 {
				lat = probeMs
			}
			out = append(out, ServerStatus{
				Backend:   p.ID,
				Name:      names[p.ID],
				Target:    s.Target(),
				Role:      s.Role,
				Healthy:   s.Healthy.Load(),
				Conns:     s.Conns.Load(),
				Requests:  st.Requests,
				BytesIn:   st.BytesIn,
				BytesOut:  st.BytesOut,
				LatencyMs: lat,
				ProbeMs:   probeMs,
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Healthy != out[j].Healthy {
			return !out[i].Healthy && out[j].Healthy
		}
		ni, nj := out[i].Name, out[j].Name
		if ni == "" {
			ni = out[i].Backend
		}
		if nj == "" {
			nj = out[j].Backend
		}
		if c := strings.Compare(strings.ToLower(ni), strings.ToLower(nj)); c != 0 {
			return c < 0
		}
		return out[i].Target < out[j].Target
	})
	return out
}

func (e *Engine) RefreshMetrics() {
	var healthy, total float64
	for _, p := range e.pools.All() {
		for _, s := range p.Servers {
			total++
			up := s.Healthy.Load()
			if up {
				healthy++
			}
			e.metrics.SetBackend(p.ID, s.Target(), up)
		}
	}
	e.metrics.SetBackendCounts(healthy, total)
}

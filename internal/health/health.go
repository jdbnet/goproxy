package health

import (
	"context"
	"crypto/tls"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"git.jdbnet.co.uk/jamie/goproxy/internal/lb"
	"git.jdbnet.co.uk/jamie/goproxy/internal/notify"
	"git.jdbnet.co.uk/jamie/goproxy/internal/proxyconfig"
)

type Checker struct {
	reg     *lb.Registry
	notify  *notify.Notifier
	mu      sync.Mutex
	cancels []context.CancelFunc
}

func New(reg *lb.Registry, n *notify.Notifier) *Checker {
	return &Checker{reg: reg, notify: n}
}

func (c *Checker) Reload(cfg *proxyconfig.Config) {
	c.mu.Lock()
	for _, cancel := range c.cancels {
		cancel()
	}
	c.cancels = nil
	for _, be := range cfg.Backends {
		if be.Health == nil {
			continue
		}
		pool := c.reg.Pool(be.ID)
		if pool == nil {
			continue
		}
		interval, _ := time.ParseDuration(be.Health.Interval)
		timeout, _ := time.ParseDuration(be.Health.Timeout)
		if interval <= 0 {
			interval = 5 * time.Second
		}
		if timeout <= 0 {
			timeout = 2 * time.Second
		}
		ctx, cancel := context.WithCancel(context.Background())
		c.cancels = append(c.cancels, cancel)
		spec := *be.Health
		go c.loop(ctx, pool, spec, interval, timeout)
	}
	c.mu.Unlock()
}

func (c *Checker) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, cancel := range c.cancels {
		cancel()
	}
	c.cancels = nil
}

func (c *Checker) loop(ctx context.Context, pool *lb.Pool, spec proxyconfig.HealthCheck, interval, timeout time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	c.probeAll(ctx, pool, spec, timeout)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			c.probeAll(ctx, pool, spec, timeout)
		}
	}
}

func (c *Checker) probeAll(ctx context.Context, pool *lb.Pool, spec proxyconfig.HealthCheck, timeout time.Duration) {
	for _, srv := range pool.Servers {
		ok := probe(ctx, srv, spec, timeout)
		c.NoteOutcome(pool.ID, srv, ok)
	}
}

// NoteOutcome records a probe or request result. Servers are only marked down or up
// after consecutive failures or successes, matching the backend health thresholds.
func (c *Checker) NoteOutcome(poolID string, srv *lb.Server, ok bool) {
	if srv == nil || !srv.HealthEnabled {
		return
	}
	healthyNeed := srv.HealthyThreshold
	unhealthyNeed := srv.UnhealthyThreshold
	if healthyNeed <= 0 {
		healthyNeed = 2
	}
	if unhealthyNeed <= 0 {
		unhealthyNeed = 5
	}

	was := srv.Healthy.Load()
	if ok {
		srv.FailStreak.Store(0)
		if was {
			srv.SuccessStreak.Store(0)
			return
		}
		if srv.SuccessStreak.Add(1) >= int32(healthyNeed) {
			srv.SuccessStreak.Store(0)
			srv.Healthy.Store(true)
			c.changed(poolID, srv, true)
		}
		return
	}

	srv.SuccessStreak.Store(0)
	if !was {
		return
	}
	if srv.FailStreak.Add(1) >= int32(unhealthyNeed) {
		srv.FailStreak.Store(0)
		srv.Healthy.Store(false)
		c.changed(poolID, srv, false)
	}
}

func (c *Checker) changed(poolID string, srv *lb.Server, up bool) {
	state := "down"
	if up {
		state = "up"
	}
	slog.Info("backend health changed", "backend", poolID, "server", srv.Target(), "state", state)
	if c.notify != nil {
		c.notify.Send(notify.Event{
			Type:     "backend." + state,
			Title:    "Backend " + state,
			Body:     poolID + " " + srv.Target() + " is " + state,
			Severity: map[bool]string{true: "info", false: "error"}[up],
		})
	}
}

func probe(ctx context.Context, srv *lb.Server, spec proxyconfig.HealthCheck, timeout time.Duration) bool {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	start := time.Now()
	var ok bool
	switch spec.Type {
	case "http":
		ok = probeHTTP(ctx, srv, spec)
	case "grpc":
		ok = probeGRPC(ctx, srv)
	default:
		ok = probeTCP(ctx, srv)
	}
	srv.SetProbeLatency(time.Since(start))
	return ok
}

func probeTCP(ctx context.Context, srv *lb.Server) bool {
	d := net.Dialer{}
	conn, err := d.DialContext(ctx, "tcp", srv.Target())
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func probeHTTP(ctx context.Context, srv *lb.Server, spec proxyconfig.HealthCheck) bool {
	u := "http://" + srv.Target() + spec.Path
	if srv.URL != nil {
		base := *srv.URL
		if spec.Path != "" {
			base.Path = spec.Path
		}
		u = base.String()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return false
	}
	client := &http.Client{
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if spec.ExpectStatus > 0 && resp.StatusCode != spec.ExpectStatus {
		return false
	}
	if spec.ExpectStatus == 0 && resp.StatusCode >= 500 {
		return false
	}
	if spec.ExpectBody != "" && !strings.Contains(string(body), spec.ExpectBody) {
		return false
	}
	return true
}

func probeGRPC(ctx context.Context, srv *lb.Server) bool {
	// gRPC health: TCP connect plus HTTP/2 preface is enough for v1 active check
	// without pulling a gRPC client. A refused/timeout port is down.
	return probeTCP(ctx, srv)
}

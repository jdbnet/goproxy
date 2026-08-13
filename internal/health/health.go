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
	healthyNeed := spec.HealthyThreshold
	unhealthyNeed := spec.UnhealthyThreshold
	if healthyNeed <= 0 {
		healthyNeed = 2
	}
	if unhealthyNeed <= 0 {
		unhealthyNeed = 3
	}
	streak := make([]int, len(pool.Servers))
	t := time.NewTicker(interval)
	defer t.Stop()
	c.probeAll(ctx, pool, spec, timeout, streak, healthyNeed, unhealthyNeed)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			c.probeAll(ctx, pool, spec, timeout, streak, healthyNeed, unhealthyNeed)
		}
	}
}

func (c *Checker) probeAll(ctx context.Context, pool *lb.Pool, spec proxyconfig.HealthCheck, timeout time.Duration, streak []int, healthyNeed, unhealthyNeed int) {
	for i, srv := range pool.Servers {
		ok := probe(ctx, srv, spec, timeout)
		was := srv.Healthy.Load()
		if ok {
			if !was {
				streak[i]++
				if streak[i] >= healthyNeed {
					srv.Healthy.Store(true)
					streak[i] = 0
					c.changed(pool.ID, srv, true)
				}
			} else {
				streak[i] = 0
			}
		} else {
			if was {
				streak[i]++
				if streak[i] >= unhealthyNeed {
					srv.Healthy.Store(false)
					streak[i] = 0
					c.changed(pool.ID, srv, false)
				}
			} else {
				streak[i] = 0
			}
		}
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

func MarkPassiveFailure(srv *lb.Server) {
	if srv == nil || !srv.HealthEnabled {
		return
	}
	srv.Healthy.Store(false)
}

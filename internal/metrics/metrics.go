package metrics

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type ScopeStats struct {
	Requests  int64   `json:"requests"`
	BytesIn   int64   `json:"bytes_in"`
	BytesOut  int64   `json:"bytes_out"`
	LatencyMs float64 `json:"latency_ms,omitempty"`
}

type Snapshot struct {
	At              time.Time             `json:"at"`
	StartedAt       time.Time             `json:"started_at,omitempty"`
	UptimeSeconds   float64               `json:"uptime_seconds,omitempty"`
	Requests        float64               `json:"requests"`
	Errors          float64               `json:"errors"`
	BytesIn         float64               `json:"bytes_in"`
	BytesOut        float64               `json:"bytes_out"`
	Connections     float64               `json:"connections"`
	LatencyMs       float64               `json:"latency_ms"`
	BackendsHealthy float64               `json:"backends_healthy"`
	BackendsTotal   float64               `json:"backends_total"`
	RequestsTotal   float64               `json:"requests_total"`
	BytesInTotal    float64               `json:"bytes_in_total"`
	BytesOutTotal   float64               `json:"bytes_out_total"`
	Frontends       map[string]ScopeStats `json:"frontends,omitempty"`
	Routes          map[string]ScopeStats `json:"routes,omitempty"`
	Backends        map[string]ScopeStats `json:"backends,omitempty"`
	Servers         map[string]ScopeStats `json:"servers,omitempty"`
}

type Metrics struct {
	Requests    prometheus.Counter
	Errors      prometheus.Counter
	BytesIn     prometheus.Counter
	BytesOut    prometheus.Counter
	Connections prometheus.Gauge
	Latency     prometheus.Histogram
	BackendUp   *prometheus.GaugeVec
	CertExpiry  *prometheus.GaugeVec

	mu         sync.Mutex
	buf        []Snapshot
	max        int
	last       Snapshot
	period     time.Duration
	started    time.Time
	totals     ScopeStats
	windowLat  float64
	windowLatN float64
	lastLatAvg float64
	byFrontend map[string]ScopeStats
	byRoute    map[string]ScopeStats
	byBackend  map[string]ScopeStats
	byServer   map[string]ScopeStats
}

var (
	promOnce      sync.Once
	promRequests  prometheus.Counter
	promErrors    prometheus.Counter
	promBytesIn   prometheus.Counter
	promBytesOut  prometheus.Counter
	promConns     prometheus.Gauge
	promLatency   prometheus.Histogram
	promBackend   *prometheus.GaugeVec
	promCert      *prometheus.GaugeVec
	promScopeReq  *prometheus.CounterVec
	promScopeByte *prometheus.CounterVec
	promServerLat *prometheus.GaugeVec
)

func New() *Metrics {
	promOnce.Do(func() {
		promRequests = promauto.NewCounter(prometheus.CounterOpts{Name: "goproxy_requests_total", Help: "Total proxied requests"})
		promErrors = promauto.NewCounter(prometheus.CounterOpts{Name: "goproxy_errors_total", Help: "Total proxy errors"})
		promBytesIn = promauto.NewCounter(prometheus.CounterOpts{Name: "goproxy_bytes_in_total", Help: "Bytes received from clients"})
		promBytesOut = promauto.NewCounter(prometheus.CounterOpts{Name: "goproxy_bytes_out_total", Help: "Bytes sent to clients"})
		promConns = promauto.NewGauge(prometheus.GaugeOpts{Name: "goproxy_connections", Help: "Open connections"})
		promLatency = promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "goproxy_request_duration_seconds",
			Help:    "Request latency",
			Buckets: prometheus.DefBuckets,
		})
		promBackend = promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "goproxy_backend_up",
			Help: "Backend server health (1=up)",
		}, []string{"backend", "server"})
		promCert = promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "goproxy_cert_expiry_seconds",
			Help: "Seconds until certificate expiry",
		}, []string{"id"})
		promScopeReq = promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "goproxy_scope_requests_total",
			Help: "Requests by frontend, route, or backend",
		}, []string{"kind", "id"})
		promScopeByte = promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "goproxy_scope_bytes_total",
			Help: "Bytes by frontend, route, or backend",
		}, []string{"kind", "id", "dir"})
		promServerLat = promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "goproxy_backend_latency_seconds",
			Help: "EWMA request latency to a backend server",
		}, []string{"backend", "server"})
	})
	return &Metrics{
		Requests:    promRequests,
		Errors:      promErrors,
		BytesIn:     promBytesIn,
		BytesOut:    promBytesOut,
		Connections: promConns,
		Latency:     promLatency,
		BackendUp:   promBackend,
		CertExpiry:  promCert,
		max:         240,
		period:      15 * time.Second,
		started:     time.Now().UTC(),
		byFrontend:  map[string]ScopeStats{},
		byRoute:     map[string]ScopeStats{},
		byBackend:   map[string]ScopeStats{},
		byServer:    map[string]ScopeStats{},
	}
}

func (m *Metrics) ObserveRequest(d time.Duration, status int, in, out int64, frontend, route, backend, server string) {
	if in < 0 {
		in = 0
	}
	if out < 0 {
		out = 0
	}
	ms := d.Seconds() * 1000
	m.Requests.Inc()
	m.Latency.Observe(d.Seconds())
	m.BytesIn.Add(float64(in))
	m.BytesOut.Add(float64(out))
	if status >= 500 {
		m.Errors.Inc()
	}
	addPromScope("frontend", frontend, in, out)
	addPromScope("route", route, in, out)
	addPromScope("backend", backend, in, out)
	m.mu.Lock()
	m.last.Requests++
	m.last.BytesIn += float64(in)
	m.last.BytesOut += float64(out)
	m.windowLat += ms
	m.windowLatN++
	if status >= 500 {
		m.last.Errors++
	}
	m.totals.Requests++
	m.totals.BytesIn += in
	m.totals.BytesOut += out
	m.byFrontend = bumpScope(m.byFrontend, frontend, in, out, ms)
	m.byRoute = bumpScope(m.byRoute, route, in, out, ms)
	m.byBackend = bumpScope(m.byBackend, backend, in, out, ms)
	if backend != "" && server != "" {
		key := serverKey(backend, server)
		m.byServer = bumpScope(m.byServer, key, in, out, ms)
		if promServerLat != nil {
			promServerLat.WithLabelValues(backend, server).Set(m.byServer[key].LatencyMs / 1000)
		}
	}
	m.mu.Unlock()
}

func addPromScope(kind, id string, in, out int64) {
	if id == "" || promScopeReq == nil {
		return
	}
	promScopeReq.WithLabelValues(kind, id).Inc()
	promScopeByte.WithLabelValues(kind, id, "in").Add(float64(in))
	promScopeByte.WithLabelValues(kind, id, "out").Add(float64(out))
}

func bumpScope(store map[string]ScopeStats, id string, in, out int64, ms float64) map[string]ScopeStats {
	if id == "" {
		return store
	}
	if store == nil {
		store = map[string]ScopeStats{}
	}
	s := store[id]
	if s.Requests == 0 {
		s.LatencyMs = ms
	} else {
		s.LatencyMs = s.LatencyMs*0.7 + ms*0.3
	}
	s.Requests++
	s.BytesIn += in
	s.BytesOut += out
	store[id] = s
	return store
}

func serverKey(backend, server string) string {
	return backend + "\x1f" + server
}

func (m *Metrics) ServerStats(backend, server string) ScopeStats {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.byServer[serverKey(backend, server)]
}

func (m *Metrics) windowLatency() float64 {
	if m.windowLatN > 0 {
		return m.windowLat / m.windowLatN
	}
	return m.lastLatAvg
}

func (m *Metrics) SetConns(n int) {
	m.Connections.Set(float64(n))
	m.mu.Lock()
	m.last.Connections = float64(n)
	m.mu.Unlock()
}

func (m *Metrics) SetBackend(backend, server string, up bool) {
	v := 0.0
	if up {
		v = 1
	}
	m.BackendUp.WithLabelValues(backend, server).Set(v)
}

func (m *Metrics) SetCertExpiry(id string, seconds float64) {
	m.CertExpiry.WithLabelValues(id).Set(seconds)
}

func (m *Metrics) SetBackendCounts(healthy, total float64) {
	m.mu.Lock()
	m.last.BackendsHealthy = healthy
	m.last.BackendsTotal = total
	m.mu.Unlock()
}

func (m *Metrics) StartBuffer(stop <-chan struct{}) {
	t := time.NewTicker(m.period)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			m.mu.Lock()
			if m.windowLatN > 0 {
				m.lastLatAvg = m.windowLat / m.windowLatN
			}
			snap := m.last
			snap.At = time.Now().UTC()
			snap.LatencyMs = m.lastLatAvg
			snap.Frontends = nil
			snap.Routes = nil
			snap.Backends = nil
			snap.Servers = nil
			m.buf = append(m.buf, snap)
			if len(m.buf) > m.max {
				m.buf = m.buf[len(m.buf)-m.max:]
			}
			m.last.Requests = 0
			m.last.Errors = 0
			m.last.BytesIn = 0
			m.last.BytesOut = 0
			m.last.LatencyMs = 0
			m.windowLat = 0
			m.windowLatN = 0
			m.mu.Unlock()
		}
	}
}

func (m *Metrics) History() []Snapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Snapshot, len(m.buf))
	copy(out, m.buf)
	return out
}

func (m *Metrics) Live() Snapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.last
	s.At = time.Now().UTC()
	s.StartedAt = m.started
	s.UptimeSeconds = time.Since(m.started).Seconds()
	s.LatencyMs = m.windowLatency()
	s.RequestsTotal = float64(m.totals.Requests)
	s.BytesInTotal = float64(m.totals.BytesIn)
	s.BytesOutTotal = float64(m.totals.BytesOut)
	s.Frontends = copyScope(m.byFrontend)
	s.Routes = copyScope(m.byRoute)
	s.Backends = copyScope(m.byBackend)
	s.Servers = copyScope(m.byServer)
	return s
}

func copyScope(in map[string]ScopeStats) map[string]ScopeStats {
	if len(in) == 0 {
		return map[string]ScopeStats{}
	}
	out := make(map[string]ScopeStats, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

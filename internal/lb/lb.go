package lb

import (
	"hash/fnv"
	"net"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"git.jdbnet.co.uk/jamie/goproxy/internal/proxyconfig"
)

type Server struct {
	ID            string
	URL           *url.URL
	Address       string
	Role          string
	Weight        int
	HealthEnabled bool
	Healthy       atomic.Bool
	Conns         atomic.Int64
	probeNs       atomic.Int64
}

func (s *Server) Target() string {
	if s.Address != "" {
		return s.Address
	}
	if s.URL != nil {
		return s.URL.Host
	}
	return ""
}

func (s *Server) SetProbeLatency(d time.Duration) {
	s.probeNs.Store(d.Nanoseconds())
}

func (s *Server) ProbeLatency() time.Duration {
	return time.Duration(s.probeNs.Load())
}

type Pool struct {
	ID        string
	Name      string
	Algorithm string
	Servers   []*Server
	mu        sync.Mutex
	rr        uint64
}

type Registry struct {
	mu    sync.RWMutex
	pools map[string]*Pool
}

func NewRegistry() *Registry {
	return &Registry{pools: map[string]*Pool{}}
}

func (r *Registry) Replace(cfg *proxyconfig.Config) {
	next := map[string]*Pool{}
	for _, be := range cfg.Backends {
		p := &Pool{ID: be.ID, Name: be.Name, Algorithm: be.Algorithm}
		for i, s := range be.Servers {
			srv := &Server{
				ID:      be.ID + "/" + itoa(i),
				Role:    s.Role,
				Weight:  s.Weight,
				Address: s.Address,
			}
			if s.URL != "" {
				u, err := url.Parse(s.URL)
				if err == nil {
					srv.URL = u
				}
			}
			srv.HealthEnabled = be.Health != nil
			srv.Healthy.Store(true)
			p.Servers = append(p.Servers, srv)
		}
		next[be.ID] = p
	}
	r.mu.Lock()
	r.pools = next
	r.mu.Unlock()
}

func (r *Registry) Pool(id string) *Pool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.pools[id]
}

func (r *Registry) All() []*Pool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Pool, 0, len(r.pools))
	for _, p := range r.pools {
		out = append(out, p)
	}
	return out
}

func (p *Pool) Pick(clientIP string) *Server {
	if p == nil {
		return nil
	}
	primaries := p.eligible("primary")
	backups := p.eligible("backup")
	cands := primaries
	if len(cands) == 0 {
		cands = backups
	}
	if len(cands) == 0 {
		return nil
	}
	switch p.Algorithm {
	case "least_conn":
		return pickLeast(cands)
	case "weighted":
		return pickWeighted(cands)
	case "ip_hash":
		return pickHash(cands, clientIP)
	default:
		return p.pickRR(cands)
	}
}

func (p *Pool) eligible(role string) []*Server {
	var out []*Server
	for _, s := range p.Servers {
		if s.Role == role && s.Healthy.Load() {
			out = append(out, s)
		}
	}
	return out
}

func (p *Pool) pickRR(cands []*Server) *Server {
	n := atomic.AddUint64(&p.rr, 1)
	return cands[int(n-1)%len(cands)]
}

func pickLeast(cands []*Server) *Server {
	best := cands[0]
	bestN := best.Conns.Load()
	for _, s := range cands[1:] {
		if n := s.Conns.Load(); n < bestN {
			best = s
			bestN = n
		}
	}
	return best
}

func pickWeighted(cands []*Server) *Server {
	total := 0
	for _, s := range cands {
		w := s.Weight
		if w <= 0 {
			w = 1
		}
		total += w
	}
	if total <= 0 {
		return cands[0]
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(cands[0].ID))
	n := int(h.Sum32()) % total
	if n < 0 {
		n = -n
	}
	acc := 0
	for _, s := range cands {
		w := s.Weight
		if w <= 0 {
			w = 1
		}
		acc += w
		if n < acc {
			return s
		}
	}
	return cands[len(cands)-1]
}

func pickHash(cands []*Server, ip string) *Server {
	h := fnv.New32a()
	_, _ = h.Write([]byte(ip))
	return cands[int(h.Sum32())%len(cands)]
}

func ClientIP(remote string) string {
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		return remote
	}
	return host
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [12]byte
	pos := len(b)
	for i > 0 {
		pos--
		b[pos] = byte('0' + i%10)
		i /= 10
	}
	return string(b[pos:])
}

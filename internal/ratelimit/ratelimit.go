package ratelimit

import (
	"sync"
	"time"

	"git.jdbnet.co.uk/jamie/goproxy/internal/proxyconfig"
)

type bucket struct {
	tokens float64
	rate   float64
	burst  float64
	last   time.Time
}

func (b *bucket) allow(now time.Time) bool {
	elapsed := now.Sub(b.last).Seconds()
	b.tokens += elapsed * b.rate
	if b.tokens > b.burst {
		b.tokens = b.burst
	}
	b.last = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

type Limiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
}

func New() *Limiter {
	return &Limiter{buckets: map[string]*bucket{}}
}

func (l *Limiter) Allow(key string, spec string) bool {
	if spec == "" {
		return true
	}
	n, window, err := proxyconfig.ParseRate(spec)
	if err != nil {
		return true
	}
	rate := n / window.Seconds()
	l.mu.Lock()
	defer l.mu.Unlock()
	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{tokens: n, rate: rate, burst: n, last: time.Now()}
		l.buckets[key] = b
	} else {
		b.rate = rate
		b.burst = n
	}
	return b.allow(time.Now())
}

func (l *Limiter) AllowACL(acl *proxyconfig.ACL, backendID, clientIP string) bool {
	if acl == nil || acl.RateLimit == nil {
		return true
	}
	rl := acl.RateLimit
	if !l.Allow("ip:"+acl.ID+":"+clientIP, rl.PerIP) {
		return false
	}
	if !l.Allow("acl:"+acl.ID, rl.PerACL) {
		return false
	}
	if !l.Allow("backend:"+backendID, rl.PerBackend) {
		return false
	}
	return true
}

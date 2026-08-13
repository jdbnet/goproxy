package lb

import (
	"testing"

	"git.jdbnet.co.uk/jamie/goproxy/internal/proxyconfig"
)

func TestRoundRobin(t *testing.T) {
	r := NewRegistry()
	r.Replace(&proxyconfig.Config{
		Backends: []proxyconfig.Backend{{
			ID:        "p",
			Algorithm: "round_robin",
			Servers: []proxyconfig.BackendServer{
				{URL: "http://a:1", Role: "primary", Weight: 1},
				{URL: "http://b:1", Role: "primary", Weight: 1},
			},
		}},
	})
	p := r.Pool("p")
	a := p.Pick("1.1.1.1")
	b := p.Pick("1.1.1.1")
	if a.URL.Host == b.URL.Host {
		t.Fatal("expected rotation")
	}
}

func TestBackupWhenPrimaryDown(t *testing.T) {
	r := NewRegistry()
	r.Replace(&proxyconfig.Config{
		Backends: []proxyconfig.Backend{{
			ID:        "p",
			Algorithm: "round_robin",
			Servers: []proxyconfig.BackendServer{
				{URL: "http://a:1", Role: "primary", Weight: 1},
				{URL: "http://b:1", Role: "backup", Weight: 1},
			},
		}},
	})
	p := r.Pool("p")
	p.Servers[0].Healthy.Store(false)
	got := p.Pick("1.1.1.1")
	if got == nil || got.URL.Host != "b:1" {
		t.Fatalf("got %+v", got)
	}
}

func TestIPHashStable(t *testing.T) {
	r := NewRegistry()
	r.Replace(&proxyconfig.Config{
		Backends: []proxyconfig.Backend{{
			ID:        "p",
			Algorithm: "ip_hash",
			Servers: []proxyconfig.BackendServer{
				{URL: "http://a:1", Role: "primary", Weight: 1},
				{URL: "http://b:1", Role: "primary", Weight: 1},
			},
		}},
	})
	p := r.Pool("p")
	first := p.Pick("10.0.0.9")
	for i := 0; i < 10; i++ {
		if p.Pick("10.0.0.9").URL.Host != first.URL.Host {
			t.Fatal("ip hash not sticky")
		}
	}
}

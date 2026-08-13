package metrics

import (
	"testing"
	"time"
)

func TestObserveRequestTotals(t *testing.T) {
	m := New()
	m.ObserveRequest(10*time.Millisecond, 200, 100, 400, "https-443", "app", "pool", "10.0.0.1:80")
	m.ObserveRequest(30*time.Millisecond, 200, 50, 50, "https-443", "app", "pool", "10.0.0.1:80")
	live := m.Live()
	if live.RequestsTotal != 2 || live.BytesInTotal != 150 || live.BytesOutTotal != 450 {
		t.Fatalf("totals %+v", live)
	}
	if live.UptimeSeconds < 0 || live.StartedAt.IsZero() {
		t.Fatalf("uptime %+v", live)
	}
	if live.LatencyMs < 19 || live.LatencyMs > 21 {
		t.Fatalf("window latency %v", live.LatencyMs)
	}
	fe := live.Frontends["https-443"]
	if fe.Requests != 2 || fe.BytesOut != 450 {
		t.Fatalf("frontend %+v", fe)
	}
	if live.Routes["app"].BytesIn != 150 {
		t.Fatalf("route %+v", live.Routes["app"])
	}
	if live.Backends["pool"].Requests != 2 {
		t.Fatalf("backend %+v", live.Backends["pool"])
	}
	srv := m.ServerStats("pool", "10.0.0.1:80")
	if srv.Requests != 2 || srv.LatencyMs <= 0 {
		t.Fatalf("server %+v", srv)
	}
}

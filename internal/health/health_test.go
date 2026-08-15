package health

import (
	"testing"

	"git.jdbnet.co.uk/jamie/goproxy/internal/lb"
	"git.jdbnet.co.uk/jamie/goproxy/internal/notify"
	"git.jdbnet.co.uk/jamie/goproxy/internal/proxyconfig"
)

func testServer() *lb.Server {
	srv := &lb.Server{HealthEnabled: true, HealthyThreshold: 2, UnhealthyThreshold: 3}
	srv.Healthy.Store(true)
	return srv
}

func TestNoteOutcomeNeedsConsecutiveFailures(t *testing.T) {
	c := New(lb.NewRegistry(), notify.New())
	srv := testServer()

	c.NoteOutcome("app", srv, false)
	if !srv.Healthy.Load() {
		t.Fatal("one failure should not mark down")
	}
	c.NoteOutcome("app", srv, false)
	if !srv.Healthy.Load() {
		t.Fatal("two failures should not mark down")
	}
	c.NoteOutcome("app", srv, false)
	if srv.Healthy.Load() {
		t.Fatal("three failures should mark down")
	}
}

func TestNoteOutcomeResetsFailureStreakOnSuccess(t *testing.T) {
	c := New(lb.NewRegistry(), notify.New())
	srv := testServer()

	c.NoteOutcome("app", srv, false)
	c.NoteOutcome("app", srv, false)
	c.NoteOutcome("app", srv, true)
	c.NoteOutcome("app", srv, false)
	c.NoteOutcome("app", srv, false)
	if !srv.Healthy.Load() {
		t.Fatal("interrupted streak should not mark down")
	}
}

func TestNoteOutcomeRecoveryNeedsConsecutiveSuccesses(t *testing.T) {
	c := New(lb.NewRegistry(), notify.New())
	srv := testServer()
	srv.Healthy.Store(false)

	c.NoteOutcome("app", srv, true)
	if srv.Healthy.Load() {
		t.Fatal("one success should not mark up")
	}
	c.NoteOutcome("app", srv, true)
	if !srv.Healthy.Load() {
		t.Fatal("two successes should mark up")
	}
}

func TestNoteOutcomePassiveMatchesActive(t *testing.T) {
	c := New(lb.NewRegistry(), notify.New())
	srv := testServer()

	for i := 0; i < 2; i++ {
		c.NoteOutcome("app", srv, false)
	}
	if !srv.Healthy.Load() {
		t.Fatal("request failures should use the same threshold as probes")
	}
	c.NoteOutcome("app", srv, false)
	if srv.Healthy.Load() {
		t.Fatal("third failure should mark down")
	}
}

func TestReplacePreservesHealthState(t *testing.T) {
	reg := lb.NewRegistry()
	reg.Replace(&proxyconfig.Config{
		Backends: []proxyconfig.Backend{{
			ID: "app",
			Servers: []proxyconfig.BackendServer{
				{URL: "http://127.0.0.1:3000", Role: "primary"},
			},
			Health: &proxyconfig.HealthCheck{Type: "tcp"},
		}},
	})
	pool := reg.Pool("app")
	pool.Servers[0].Healthy.Store(false)
	pool.Servers[0].FailStreak.Store(2)

	reg.Replace(&proxyconfig.Config{
		Backends: []proxyconfig.Backend{{
			ID: "app",
			Servers: []proxyconfig.BackendServer{
				{URL: "http://127.0.0.1:3000", Role: "primary"},
			},
			Health: &proxyconfig.HealthCheck{Type: "tcp"},
		}},
	})
	got := reg.Pool("app").Servers[0]
	if got.Healthy.Load() {
		t.Fatal("expected health state to survive pool replace")
	}
	if got.FailStreak.Load() != 2 {
		t.Fatalf("expected fail streak preserved, got %d", got.FailStreak.Load())
	}
}

func TestNoteOutcomeDisabledHealth(t *testing.T) {
	c := New(lb.NewRegistry(), notify.New())
	srv := &lb.Server{HealthEnabled: false}
	srv.Healthy.Store(true)

	for i := 0; i < 10; i++ {
		c.NoteOutcome("app", srv, false)
	}
	if !srv.Healthy.Load() {
		t.Fatal("disabled health checks should not change state")
	}
}

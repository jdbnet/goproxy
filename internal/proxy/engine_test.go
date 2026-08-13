package proxy

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"git.jdbnet.co.uk/jamie/goproxy/internal/config"
	"git.jdbnet.co.uk/jamie/goproxy/internal/health"
	"git.jdbnet.co.uk/jamie/goproxy/internal/lb"
	"git.jdbnet.co.uk/jamie/goproxy/internal/metrics"
	"git.jdbnet.co.uk/jamie/goproxy/internal/notify"
	"git.jdbnet.co.uk/jamie/goproxy/internal/proxyconfig"
	"git.jdbnet.co.uk/jamie/goproxy/internal/tlsx"
)

func TestL7Proxy(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok:" + r.Host))
	}))
	defer backend.Close()

	app := &config.Config{DataDir: t.TempDir(), ACMEDirectory: "https://acme-staging-v02.api.letsencrypt.org/directory"}
	n := notify.New()
	m := metrics.New()
	pools := lb.NewRegistry()
	hc := health.New(pools, n)
	certs := tlsx.NewStore(app, n, m)
	eng := New(certs, pools, hc, m, n)

	ln := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	ln.Close()
	bind := ln.Listener.Addr().String()

	cfg := &proxyconfig.Config{
		Frontends: []proxyconfig.Frontend{{ID: "http", Bind: bind}},
		ACLs: []proxyconfig.ACL{{
			ID: "app", Frontend: "http", Match: proxyconfig.Match{Host: "app.test"},
			Mode: "terminate", Backend: "pool",
		}},
		Backends: []proxyconfig.Backend{{
			ID: "pool", Algorithm: "round_robin",
			Servers: []proxyconfig.BackendServer{{URL: backend.URL, Role: "primary", Weight: 1}},
		}},
	}
	if err := eng.Apply(cfg); err != nil {
		t.Fatal(err)
	}
	defer eng.Shutdown(context.Background())
	time.Sleep(50 * time.Millisecond)

	req, _ := http.NewRequest(http.MethodGet, "http://"+bind+"/", nil)
	req.Host = "app.test"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "ok:app.test" {
		t.Fatalf("body %q", body)
	}
}

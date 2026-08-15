package proxy

import (
	"context"
	"crypto/tls"
	"io"
	"net"
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

func TestPassthrough(t *testing.T) {
	backendLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer backendLn.Close()
	go func() {
		for {
			c, err := backendLn.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				buf := make([]byte, 1024)
				n, _ := c.Read(buf)
				if n > 0 {
					_, _ = c.Write([]byte("copied"))
				}
			}(c)
		}
	}()

	app := &config.Config{DataDir: t.TempDir()}
	n := notify.New()
	m := metrics.New()
	pools := lb.NewRegistry()
	hc := health.New(pools, n)
	certs := tlsx.NewStore(app, n, m)
	eng := New(app, certs, pools, hc, m, n)

	feLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	bind := feLn.Addr().String()
	feLn.Close()

	cfg := &proxyconfig.Config{
		Frontends: []proxyconfig.Frontend{{ID: "https", Bind: bind, TLS: &proxyconfig.FrontendTLS{Enabled: true}}},
		ACLs: []proxyconfig.ACL{{
			ID: "mail", Frontend: "https", Match: proxyconfig.Match{Host: "mail.test"},
			Mode: "passthrough", Backend: "mail",
		}},
		Backends: []proxyconfig.Backend{{
			ID: "mail", Algorithm: "round_robin",
			Servers: []proxyconfig.BackendServer{{Address: backendLn.Addr().String(), Role: "primary", Weight: 1}},
		}},
	}
	if err := eng.Apply(cfg); err != nil {
		t.Fatal(err)
	}
	defer eng.Shutdown(context.Background())
	time.Sleep(50 * time.Millisecond)

	hello := clientHello("mail.test")
	conn, err := net.Dial("tcp", bind)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := conn.Write(hello); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 16)
	nread, err := conn.Read(buf)
	if err != nil && err != io.EOF {
		t.Fatal(err)
	}
	if string(buf[:nread]) != "copied" {
		t.Fatalf("got %q", buf[:nread])
	}
}

func clientHello(sni string) []byte {
	c, s := net.Pipe()
	defer s.Close()
	go func() {
		tlsConn := tls.Client(c, &tls.Config{ServerName: sni, InsecureSkipVerify: true})
		_ = tlsConn.Handshake()
		_ = tlsConn.Close()
	}()
	buf := make([]byte, 4096)
	n, _ := s.Read(buf)
	_ = c.Close()
	return buf[:n]
}

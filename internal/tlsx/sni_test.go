package tlsx

import (
	"crypto/tls"
	"io"
	"net"
	"testing"
	"time"
)

func TestPeekSNI(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	done := make(chan string, 1)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			done <- ""
			return
		}
		defer c.Close()
		_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
		sni, _, err := PeekSNI(c)
		if err != nil {
			done <- ""
			return
		}
		done <- sni
	}()

	conn, err := tls.Dial("tcp", ln.Addr().String(), &tls.Config{
		ServerName:         "app.example.com",
		InsecureSkipVerify: true,
	})
	if err == nil {
		conn.Close()
	}
	select {
	case sni := <-done:
		if sni != "app.example.com" {
			t.Fatalf("sni %q", sni)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout")
	}
}

func TestPeekNotTLS(t *testing.T) {
	r, w := io.Pipe()
	go func() {
		_, _ = w.Write([]byte("GET / HTTP/1.1\r\n\r\n"))
		_ = w.Close()
	}()
	_, _, err := PeekSNI(r)
	if err == nil {
		t.Fatal("expected error")
	}
}

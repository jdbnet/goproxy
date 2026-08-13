package notify

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"git.jdbnet.co.uk/jamie/goproxy/internal/proxyconfig"
)

func TestMatchTrigger(t *testing.T) {
	if !matchTrigger(nil, "backend.down") {
		t.Fatal("empty triggers should match all")
	}
	if !matchTrigger([]string{"*"}, "git.conflict") {
		t.Fatal("* should match")
	}
	if !matchTrigger([]string{"backend.*"}, "backend.up") {
		t.Fatal("prefix should match")
	}
	if matchTrigger([]string{"backend.up"}, "backend.down") {
		t.Fatal("exact mismatch")
	}
}

func TestDeliverGeneric(t *testing.T) {
	var got Event
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("content-type %s", r.Header.Get("Content-Type"))
		}
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	n := New()
	status, err := n.Deliver(proxyconfig.Webhook{URL: srv.URL, Format: "generic"}, Event{
		Type: "backend.down", Title: "Backend down", Body: "pool target is down", Severity: "error",
	})
	if err != nil {
		t.Fatal(err)
	}
	if status != http.StatusNoContent {
		t.Fatalf("status %d", status)
	}
	if got.Type != "backend.down" || got.Title == "" {
		t.Fatalf("payload %+v", got)
	}
}

func TestTestSendsSample(t *testing.T) {
	var got Event
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	n := New()
	if err := n.Test(proxyconfig.Webhook{URL: srv.URL}); err != nil {
		t.Fatal(err)
	}
	if got.Type != "test" {
		t.Fatalf("type %s", got.Type)
	}
}

func TestDeliverRejectsBadURL(t *testing.T) {
	n := New()
	if _, err := n.Deliver(proxyconfig.Webhook{URL: "ftp://example"}, Event{}); err == nil {
		t.Fatal("expected error")
	}
}

func TestDeliverDiscordStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if _, ok := payload["embeds"]; !ok {
			t.Fatal("missing embeds")
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	n := New()
	if _, err := n.Deliver(proxyconfig.Webhook{URL: srv.URL, Format: "discord"}, Event{
		Title: "GoProxy test", Body: "ok", Severity: "info",
	}); err != nil {
		t.Fatal(err)
	}
}

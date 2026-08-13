package auth

import (
	"testing"

	"git.jdbnet.co.uk/jamie/goproxy/internal/store"
)

func TestUserAndAPIKey(t *testing.T) {
	db, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := New(db.SQL)
	u, err := s.CreateUser("admin", "secret", RoleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.Authenticate("admin", "secret")
	if err != nil || got.ID != u.ID {
		t.Fatalf("auth: %v %+v", err, got)
	}
	if _, err := s.Authenticate("admin", "nope"); err == nil {
		t.Fatal("expected fail")
	}
	token, key, err := s.CreateAPIKey("ci", []string{"stats:read"}, &u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if key.Prefix == "" || token == "" {
		t.Fatal("empty key")
	}
	looked, err := s.LookupAPIKey(token)
	if err != nil || looked.ID != key.ID {
		t.Fatalf("lookup: %v %+v", err, looked)
	}
}

package auth

import (
	"testing"

	"git.jdbnet.co.uk/jamie/goproxy/internal/store"
)

func TestBootstrapDefaultAdmin(t *testing.T) {
	db, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := New(db.SQL)
	username, created, err := s.BootstrapAdmin("", "")
	if err != nil || !created || username != DefaultAdminUser {
		t.Fatalf("bootstrap: created=%v username=%q err=%v", created, username, err)
	}
	if _, err := s.Authenticate(DefaultAdminUser, DefaultAdminPassword); err != nil {
		t.Fatalf("default login: %v", err)
	}
	_, created, err = s.BootstrapAdmin("", "")
	if err != nil || created {
		t.Fatalf("second bootstrap: created=%v err=%v", created, err)
	}
}

func TestBootstrapAdminEnvOverride(t *testing.T) {
	db, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := New(db.SQL)
	username, created, err := s.BootstrapAdmin("root", "s3cret")
	if err != nil || !created || username != "root" {
		t.Fatalf("bootstrap: created=%v username=%q err=%v", created, username, err)
	}
	if _, err := s.Authenticate("root", "s3cret"); err != nil {
		t.Fatalf("env login: %v", err)
	}
}

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
	updated, err := s.UpdateAPIKeyScopes(key.ID, []string{"stats:read", "frontends:read"})
	if err != nil {
		t.Fatal(err)
	}
	if len(updated.Scopes) != 2 {
		t.Fatalf("scopes: %+v", updated.Scopes)
	}
	looked, err = s.LookupAPIKey(token)
	if err != nil {
		t.Fatal(err)
	}
	if len(looked.Scopes) != 2 {
		t.Fatalf("lookup scopes after update: %+v", looked.Scopes)
	}
}

func TestChangePassword(t *testing.T) {
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
	if err := s.ChangePassword(u.ID, "nope", "newer"); err == nil {
		t.Fatal("expected current password check to fail")
	}
	if err := s.ChangePassword(u.ID, "secret", "secret"); err == nil {
		t.Fatal("expected same-password reject")
	}
	if err := s.ChangePassword(u.ID, "secret", "newer"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Authenticate("admin", "newer"); err != nil {
		t.Fatalf("new password: %v", err)
	}
	if _, err := s.Authenticate("admin", "secret"); err == nil {
		t.Fatal("old password still works")
	}
}

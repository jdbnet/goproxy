package config

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestParseLogLevel(t *testing.T) {
	cases := map[string]slog.Level{
		"":        slog.LevelInfo,
		"info":    slog.LevelInfo,
		"DEBUG":   slog.LevelDebug,
		"warn":    slog.LevelWarn,
		"warning": slog.LevelWarn,
		"error":   slog.LevelError,
	}
	for in, want := range cases {
		if got := ParseLogLevel(in); got != want {
			t.Fatalf("%q: got %v want %v", in, got, want)
		}
	}
}

func TestAccessLogsEnabled(t *testing.T) {
	cfg := defaultConfig()
	if cfg.AccessLogsEnabled() {
		t.Fatal("expected access logs off by default")
	}
	cfg.LogRequests = true
	if !cfg.AccessLogsEnabled() {
		t.Fatal("expected access logs when log_requests is true")
	}
	cfg.LogRequests = false
	cfg.LogLevel = "debug"
	if !cfg.AccessLogsEnabled() {
		t.Fatal("expected access logs when log_level is debug")
	}
}

func TestValidateACMEEmail(t *testing.T) {
	if err := ValidateACMEEmail(""); err != nil {
		t.Fatal(err)
	}
	if err := ValidateACMEEmail("admin@example.com"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateACMEEmail("not-an-email"); err == nil {
		t.Fatal("expected invalid email error")
	}
}

func TestValidateGit(t *testing.T) {
	if err := ValidateGit(GitConfig{Enabled: false}); err != nil {
		t.Fatal(err)
	}
	err := ValidateGit(GitConfig{Enabled: true, URL: "git@example.com:repo.git", Auth: "ssh"})
	if err == nil {
		t.Fatal("expected key_path error")
	}
	err = ValidateGit(GitConfig{Enabled: true, URL: "https://example.com/repo.git", Auth: "token"})
	if err == nil {
		t.Fatal("expected token error")
	}
	if err := ValidateGit(GitConfig{Enabled: true, URL: "git@example.com:repo.git", Auth: "ssh", KeyPath: "/tmp/key"}); err != nil {
		t.Fatal(err)
	}
}

func TestSaveRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	cfg := defaultConfig()
	cfg.ACMEEmail = "admin@example.com"
	cfg.Git.Enabled = true
	cfg.Git.URL = "git@example.com:repo.git"
	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ACMEEmail != cfg.ACMEEmail {
		t.Fatalf("email = %q", loaded.ACMEEmail)
	}
	if !loaded.Git.Enabled || loaded.Git.URL != cfg.Git.URL {
		t.Fatalf("git = %+v", loaded.Git)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}

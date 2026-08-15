package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Listen        string       `yaml:"listen"`
	DataDir       string       `yaml:"data_dir"`
	ProxyConfig   string       `yaml:"proxy_config"`
	ACMEEmail     string       `yaml:"acme_email"`
	ACMEDirectory string       `yaml:"acme_directory"`
	Git           GitConfig    `yaml:"git"`
	Backup        BackupConfig `yaml:"backup"`
	TLS           TLSConfig    `yaml:"tls"`
	Update        UpdateConfig `yaml:"update"`
	LogLevel      string       `yaml:"log_level"`
}

type UpdateConfig struct {
	Enabled  bool   `yaml:"enabled"`
	URL      string `yaml:"url"`
	AllowDev bool   `yaml:"allow_dev"`
}

type GitConfig struct {
	Enabled bool   `yaml:"enabled"`
	URL     string `yaml:"url"`
	Branch  string `yaml:"branch"`
	Auth    string `yaml:"auth"`
	KeyPath string `yaml:"key_path"`
	Token   string `yaml:"token"`
}

type BackupConfig struct {
	Enabled       bool   `yaml:"enabled"`
	Schedule      string `yaml:"schedule"`
	RetentionDays int    `yaml:"retention_days"`
	Dest          string `yaml:"dest"`
	Dir           string `yaml:"dir"`
}

type TLSConfig struct {
	RenewCheck   Duration `yaml:"renew_check"`
	RenewBefore  Duration `yaml:"renew_before"`
	WarnBefore   Duration `yaml:"warn_before"`
	Jitter       Duration `yaml:"jitter"`
	RetryBackoff Duration `yaml:"retry_backoff"`
	RetryMax     Duration `yaml:"retry_max"`
}

type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err != nil {
		return err
	}
	parsed, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	d.Duration = parsed
	return nil
}

func (d Duration) MarshalYAML() (any, error) {
	return d.Duration.String(), nil
}

func defaultConfig() *Config {
	return &Config{
		Listen:        "127.0.0.1:8080",
		DataDir:       "/var/lib/goproxy",
		ProxyConfig:   "/etc/goproxy/proxy.yaml",
		ACMEDirectory: "https://acme-v02.api.letsencrypt.org/directory",
		Git: GitConfig{
			Branch:  "main",
			Auth:    "ssh",
			KeyPath: "/etc/goproxy/deploy_key",
		},
		Backup: BackupConfig{
			Enabled:       true,
			Schedule:      "0 3 * * *",
			RetentionDays: 14,
			Dest:          "dir",
			Dir:           "/var/lib/goproxy/backups",
		},
		TLS: TLSConfig{
			RenewCheck:   Duration{24 * time.Hour},
			RenewBefore:  Duration{30 * 24 * time.Hour},
			WarnBefore:   Duration{14 * 24 * time.Hour},
			Jitter:       Duration{time.Hour},
			RetryBackoff: Duration{15 * time.Minute},
			RetryMax:     Duration{8 * time.Hour},
		},
		Update: UpdateConfig{
			Enabled: true,
		},
		LogLevel: "info",
	}
}

func (c *Config) SlogLevel() slog.Level {
	return ParseLogLevel(c.LogLevel)
}

func ParseLogLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func Load(path string) (*Config, error) {
	cfg := defaultConfig()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			applyEnv(cfg)
			return cfg, nil
		}
		return nil, fmt.Errorf("read config: %w", err)
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	applyEnv(cfg)
	return cfg, nil
}

func applyEnv(cfg *Config) {
	if v := os.Getenv("GOPROXY_LISTEN"); v != "" {
		cfg.Listen = v
	}
	if v := os.Getenv("GOPROXY_DATA_DIR"); v != "" {
		cfg.DataDir = v
	}
	if v := os.Getenv("GOPROXY_PROXY_CONFIG"); v != "" {
		cfg.ProxyConfig = v
	}
	if v := os.Getenv("GOPROXY_ACME_EMAIL"); v != "" {
		cfg.ACMEEmail = v
	}
	if v := os.Getenv("GOPROXY_ACME_DIRECTORY"); v != "" {
		cfg.ACMEDirectory = v
	}
	if v := os.Getenv("GOPROXY_GIT_ENABLED"); v != "" {
		cfg.Git.Enabled = parseBool(v)
	}
	if v := os.Getenv("GOPROXY_GIT_URL"); v != "" {
		cfg.Git.URL = v
	}
	if v := os.Getenv("GOPROXY_GIT_BRANCH"); v != "" {
		cfg.Git.Branch = v
	}
	if v := os.Getenv("GOPROXY_GIT_AUTH"); v != "" {
		cfg.Git.Auth = v
	}
	if v := os.Getenv("GOPROXY_GIT_KEY_PATH"); v != "" {
		cfg.Git.KeyPath = v
	}
	if v := os.Getenv("GOPROXY_GIT_TOKEN"); v != "" {
		cfg.Git.Token = v
	}
	if v := os.Getenv("GOPROXY_UPDATE_ENABLED"); v != "" {
		cfg.Update.Enabled = parseBool(v)
	}
	if v := os.Getenv("GOPROXY_UPDATE_URL"); v != "" {
		cfg.Update.URL = v
	}
	if v := os.Getenv("GOPROXY_LOG_LEVEL"); v != "" {
		cfg.LogLevel = v
	}
}

func parseBool(v string) bool {
	b, err := strconv.ParseBool(strings.TrimSpace(v))
	return err == nil && b
}

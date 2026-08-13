package backup

import (
	"compress/gzip"
	"context"
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"git.jdbnet.co.uk/jamie/goproxy/internal/config"
	"git.jdbnet.co.uk/jamie/goproxy/internal/gitsync"
	"git.jdbnet.co.uk/jamie/goproxy/internal/notify"
	"github.com/robfig/cron/v3"
)

type Manager struct {
	cfg    config.BackupConfig
	dbPath string
	db     *sql.DB
	git    *gitsync.Sync
	notify *notify.Notifier
	cron   *cron.Cron
}

func New(app *config.Config, db *sql.DB, dbPath string, git *gitsync.Sync, n *notify.Notifier) *Manager {
	return &Manager{cfg: app.Backup, db: db, dbPath: dbPath, git: git, notify: n}
}

func (m *Manager) Start() error {
	if !m.cfg.Enabled {
		return nil
	}
	if m.cfg.Schedule == "" {
		m.cfg.Schedule = "0 3 * * *"
	}
	m.cron = cron.New()
	_, err := m.cron.AddFunc(m.cfg.Schedule, func() {
		if err := m.Run(context.Background()); err != nil {
			slog.Error("state backup failed", "err", err)
			if m.notify != nil {
				m.notify.Send(notify.Event{Type: "backup.failure", Title: "State backup failed", Body: err.Error(), Severity: "error"})
			}
		}
	})
	if err != nil {
		return err
	}
	m.cron.Start()
	return nil
}

func (m *Manager) Stop() {
	if m.cron != nil {
		m.cron.Stop()
	}
}

func (m *Manager) Run(ctx context.Context) error {
	destDir := m.cfg.Dir
	if m.cfg.Dest == "git" && m.git != nil {
		destDir = filepath.Join(m.git.WorkDir(), "backups")
	}
	if destDir == "" {
		destDir = filepath.Join(filepath.Dir(m.dbPath), "backups")
	}
	if err := os.MkdirAll(destDir, 0o700); err != nil {
		return err
	}
	stamp := time.Now().UTC().Format("20060102T150405")
	raw := filepath.Join(destDir, "state-"+stamp+".db")
	gzPath := raw + ".gz"

	if _, err := m.db.ExecContext(ctx, `VACUUM INTO ?`, raw); err != nil {
		return fmt.Errorf("vacuum into: %w", err)
	}
	if err := gzipFile(raw, gzPath); err != nil {
		_ = os.Remove(raw)
		return err
	}
	_ = os.Remove(raw)

	if err := m.prune(destDir); err != nil {
		return err
	}
	if m.cfg.Dest == "git" && m.git != nil && m.git.Enabled() {
		if _, err := m.git.CommitPush("backup: state.db " + stamp); err != nil {
			return err
		}
	}
	slog.Info("state backup complete", "path", gzPath)
	return nil
}

func (m *Manager) prune(dir string) error {
	days := m.cfg.RetentionDays
	if days <= 0 {
		days = 14
	}
	cutoff := time.Now().Add(-time.Duration(days) * 24 * time.Hour)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), "state-") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			_ = os.Remove(filepath.Join(dir, e.Name()))
		}
	}
	return nil
}

func gzipFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	zw := gzip.NewWriter(out)
	if _, err := io.Copy(zw, in); err != nil {
		_ = zw.Close()
		return err
	}
	return zw.Close()
}

package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"git.jdbnet.co.uk/jamie/goproxy/internal/api"
	"git.jdbnet.co.uk/jamie/goproxy/internal/audit"
	"git.jdbnet.co.uk/jamie/goproxy/internal/auth"
	"git.jdbnet.co.uk/jamie/goproxy/internal/backup"
	"git.jdbnet.co.uk/jamie/goproxy/internal/config"
	"git.jdbnet.co.uk/jamie/goproxy/internal/gitsync"
	"git.jdbnet.co.uk/jamie/goproxy/internal/health"
	"git.jdbnet.co.uk/jamie/goproxy/internal/lb"
	"git.jdbnet.co.uk/jamie/goproxy/internal/metrics"
	"git.jdbnet.co.uk/jamie/goproxy/internal/notify"
	"git.jdbnet.co.uk/jamie/goproxy/internal/proxy"
	"git.jdbnet.co.uk/jamie/goproxy/internal/proxyconfig"
	"git.jdbnet.co.uk/jamie/goproxy/internal/store"
	"git.jdbnet.co.uk/jamie/goproxy/internal/tlsx"
	"git.jdbnet.co.uk/jamie/goproxy/internal/update"
)

var Version = "dev"

func main() {
	showVersion := flag.Bool("version", false, "print version")
	flag.Parse()
	if *showVersion {
		fmt.Println(Version)
		return
	}

	configPath := "config.yaml"
	if args := flag.Args(); len(args) > 0 {
		configPath = args[0]
	}

	app, err := config.Load(configPath)
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: app.SlogLevel()})))
	if err := os.MkdirAll(app.DataDir, 0o755); err != nil {
		slog.Error("data_dir", "err", err)
		os.Exit(1)
	}

	updCtx, updCancel := context.WithTimeout(context.Background(), 3*time.Minute)
	replaced, err := update.Check(updCtx, update.Config{
		Enabled:  app.Update.Enabled,
		URL:      app.Update.URL,
		AllowDev: app.Update.AllowDev,
	}, Version, app.DataDir)
	updCancel()
	if err != nil {
		slog.Warn("update check failed, continuing", "err", err)
	} else if replaced {
		slog.Info("installed newer binary, restarting")
		if err := update.Restart(); err != nil {
			slog.Error("restart after update", "err", err)
		}
	}

	db, err := store.Open(app.DataDir)
	if err != nil {
		slog.Error("store", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	authSvc := auth.New(db.SQL)
	if err := authSvc.BootstrapAdmin(os.Getenv("GOPROXY_ADMIN_USER"), os.Getenv("GOPROXY_ADMIN_PASSWORD")); err != nil {
		slog.Error("bootstrap admin", "err", err)
		os.Exit(1)
	}
	hasUsers, err := auth.HasUsers(db.SQL)
	if err != nil {
		slog.Error("users", "err", err)
		os.Exit(1)
	}

	al := audit.New(db.SQL)
	n := notify.New()
	m := metrics.New()
	pools := lb.NewRegistry()
	hc := health.New(pools, n)
	certs := tlsx.NewStore(app, n, m)
	eng := proxy.New(certs, pools, hc, m, n)

	git, err := gitsync.New(app)
	if err != nil {
		slog.Error("git", "err", err)
		os.Exit(1)
	}

	cfgSvc := api.NewConfigService(app.ProxyConfig, eng, git, al, n)

	if git.Enabled() {
		sha, err := git.Pull()
		if err != nil {
			slog.Error("git pull failed, using local proxy.yaml", "err", err)
			n.Send(notify.Event{Type: "git.conflict", Title: "Git pull failed on startup", Body: err.Error(), Severity: "error"})
		} else if sha != "" {
			if err := cfgSvc.ApplyFromDisk("system:gitsync", "gitsync", sha); err != nil {
				slog.Error("apply after git pull", "err", err)
			}
		}
	}

	if eng.Config() == nil {
		pcfg, created, err := proxyconfig.LoadOrCreate(app.ProxyConfig)
		if err != nil {
			slog.Error("proxy config", "err", err)
			os.Exit(1)
		}
		if created {
			slog.Info("created default proxy config", "path", app.ProxyConfig)
		}
		if err := eng.Apply(pcfg); err != nil {
			slog.Error("apply proxy config", "err", err)
			os.Exit(1)
		}
	}

	bak := backup.New(app, db.SQL, db.Path, git, n)
	if err := bak.Start(); err != nil {
		slog.Error("backup scheduler", "err", err)
		os.Exit(1)
	}
	defer bak.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go certs.StartRenewer(ctx)
	stopBuf := make(chan struct{})
	go m.StartBuffer(stopBuf)
	go func() {
		t := time.NewTicker(15 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				eng.RefreshMetrics()
			}
		}
	}()

	srvAPI := api.New(app, configPath, authSvc, al, cfgSvc, eng, certs, m, bak, git, n, hasUsers, Version)
	httpSrv := &http.Server{
		Addr:              app.Listen,
		Handler:           srvAPI.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		slog.Info("management listening", "addr", app.Listen, "version", Version)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("admin server", "err", err)
			os.Exit(1)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	slog.Info("shutting down")
	cancel()
	close(stopBuf)
	shctx, shcancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shcancel()
	_ = httpSrv.Shutdown(shctx)
	_ = eng.Shutdown(shctx)
}

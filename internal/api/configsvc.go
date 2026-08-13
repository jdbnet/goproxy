package api

import (
	"fmt"
	"sync"

	"git.jdbnet.co.uk/jamie/goproxy/internal/audit"
	"git.jdbnet.co.uk/jamie/goproxy/internal/auth"
	"git.jdbnet.co.uk/jamie/goproxy/internal/gitsync"
	"git.jdbnet.co.uk/jamie/goproxy/internal/notify"
	"git.jdbnet.co.uk/jamie/goproxy/internal/proxy"
	"git.jdbnet.co.uk/jamie/goproxy/internal/proxyconfig"
)

type ConfigService struct {
	path   string
	engine *proxy.Engine
	git    *gitsync.Sync
	audit  *audit.Log
	notify *notify.Notifier
	mu     sync.Mutex
}

func NewConfigService(path string, engine *proxy.Engine, git *gitsync.Sync, al *audit.Log, n *notify.Notifier) *ConfigService {
	return &ConfigService{path: path, engine: engine, git: git, audit: al, notify: n}
}

func (s *ConfigService) Current() *proxyconfig.Config {
	if cfg := s.engine.Config(); cfg != nil {
		return cfg.Clone()
	}
	return &proxyconfig.Config{}
}

func (s *ConfigService) Mutate(actor *auth.Actor, action, resource string, before, after any, fn func(*proxyconfig.Config) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.Current()
	if err := fn(next); err != nil {
		return err
	}
	if err := next.Validate(); err != nil {
		return err
	}
	if err := s.engine.Apply(next); err != nil {
		return err
	}
	if err := proxyconfig.Write(s.path, next); err != nil {
		return err
	}
	s.record(actor, action, resource, before, after)
	if s.git != nil && s.git.Enabled() {
		if _, err := s.git.CommitPush(action + " " + resource); err != nil {
			return fmt.Errorf("git push: %w", err)
		}
	}
	return nil
}

func (s *ConfigService) Replace(actor *auth.Actor, next *proxyconfig.Config) error {
	return s.Mutate(actor, "config.replace", "config", s.Current(), next, func(c *proxyconfig.Config) error {
		*c = *next
		return nil
	})
}

func (s *ConfigService) ApplyFromDisk(actorType, actorID, sha string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cfg, err := proxyconfig.Load(s.path)
	if err != nil {
		return err
	}
	if err := cfg.Validate(); err != nil {
		return err
	}
	if err := s.engine.Apply(cfg); err != nil {
		return err
	}
	msg := "config reloaded from disk"
	if sha != "" {
		msg = "config reloaded from Git commit " + sha
	}
	if s.audit != nil {
		_ = s.audit.Record(actorType, actorID, msg, "config", nil, map[string]string{"sha": sha})
	}
	return nil
}

func (s *ConfigService) record(actor *auth.Actor, action, resource string, before, after any) {
	if s.audit == nil {
		return
	}
	typ, id := audit.ActorSystem, "goproxy"
	if actor != nil {
		typ, id = actor.Type, actor.ID
	}
	_ = s.audit.Record(typ, id, action, resource, before, after)
}

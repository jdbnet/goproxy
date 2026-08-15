package gitsync

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"git.jdbnet.co.uk/jamie/goproxy/internal/config"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
)

type Sync struct {
	cfg  config.GitConfig
	dir  string
	auth transport.AuthMethod
}

func New(app *config.Config) (*Sync, error) {
	s := &Sync{cfg: app.Git, dir: filepath.Dir(app.ProxyConfig)}
	if !app.Git.Enabled {
		return s, nil
	}
	auth, err := s.authMethod()
	if err != nil {
		return nil, err
	}
	s.auth = auth
	return s, nil
}

func (s *Sync) Reconfigure(cfg config.GitConfig) error {
	s.cfg = cfg
	if !s.Enabled() {
		s.auth = nil
		return nil
	}
	auth, err := s.authMethod()
	if err != nil {
		return err
	}
	s.auth = auth
	return nil
}

func (s *Sync) Enabled() bool { return s.cfg.Enabled && s.cfg.URL != "" }

func (s *Sync) authMethod() (transport.AuthMethod, error) {
	switch s.cfg.Auth {
	case "token":
		return &http.BasicAuth{Username: "git", Password: s.cfg.Token}, nil
	default:
		if s.cfg.KeyPath == "" {
			return nil, fmt.Errorf("git key_path required for ssh auth")
		}
		return ssh.NewPublicKeysFromFile("git", s.cfg.KeyPath, "")
	}
}

func (s *Sync) Pull() (string, error) {
	if !s.Enabled() {
		return "", nil
	}
	repo, err := s.openOrClone()
	if err != nil {
		return "", err
	}
	wt, err := repo.Worktree()
	if err != nil {
		return "", err
	}
	err = wt.Pull(&git.PullOptions{
		RemoteName:    "origin",
		ReferenceName: plumbing.NewBranchReferenceName(s.branch()),
		Auth:          s.auth,
	})
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return "", err
	}
	return s.Head(repo)
}

func (s *Sync) CommitPush(message string) (string, error) {
	if !s.Enabled() {
		return "", nil
	}
	repo, err := s.openOrClone()
	if err != nil {
		return "", err
	}
	wt, err := repo.Worktree()
	if err != nil {
		return "", err
	}
	if err := wt.AddWithOptions(&git.AddOptions{All: true}); err != nil {
		return "", err
	}
	st, err := wt.Status()
	if err != nil {
		return "", err
	}
	if st.IsClean() {
		return s.Head(repo)
	}
	hash, err := wt.Commit(message, &git.CommitOptions{
		Author: &object.Signature{Name: "goproxy", Email: "goproxy@local", When: time.Now()},
	})
	if err != nil {
		return "", err
	}
	if err := repo.Push(&git.PushOptions{Auth: s.auth}); err != nil && err != git.NoErrAlreadyUpToDate {
		return hash.String(), err
	}
	return hash.String(), nil
}

func (s *Sync) Head(repo *git.Repository) (string, error) {
	if repo == nil {
		var err error
		repo, err = git.PlainOpen(s.dir)
		if err != nil {
			return "", err
		}
	}
	ref, err := repo.Head()
	if err != nil {
		return "", err
	}
	return ref.Hash().String(), nil
}

func (s *Sync) openOrClone() (*git.Repository, error) {
	repo, err := git.PlainOpen(s.dir)
	if err == nil {
		return repo, nil
	}
	if !os.IsNotExist(err) && err != git.ErrRepositoryNotExists {
		if _, statErr := os.Stat(filepath.Join(s.dir, ".git")); statErr == nil {
			return nil, err
		}
	}
	return git.PlainClone(s.dir, false, &git.CloneOptions{
		URL:           s.cfg.URL,
		Auth:          s.auth,
		ReferenceName: plumbing.NewBranchReferenceName(s.branch()),
		SingleBranch:  true,
	})
}

func (s *Sync) branch() string {
	if s.cfg.Branch == "" {
		return "main"
	}
	return s.cfg.Branch
}

func (s *Sync) WorkDir() string { return s.dir }

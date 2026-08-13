package tlsx

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type credStore struct {
	mu    sync.Mutex
	path  string
	creds map[string]map[string]string
}

func newCredStore(dataDir string) *credStore {
	return &credStore{
		path:  filepath.Join(dataDir, "dns-creds.json"),
		creds: map[string]map[string]string{},
	}
}

func (c *credStore) load() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	raw, err := os.ReadFile(c.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return json.Unmarshal(raw, &c.creds)
}

func (c *credStore) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(c.path), 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(c.creds, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.path, raw, 0o600)
}

func (c *credStore) Set(id string, creds map[string]string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.creds == nil {
		c.creds = map[string]map[string]string{}
	}
	merged := c.creds[id]
	if merged == nil {
		merged = map[string]string{}
	}
	for k, v := range creds {
		if strings.TrimSpace(v) == "" {
			continue
		}
		merged[k] = v
	}
	c.creds[id] = merged
	return c.saveLocked()
}

func (c *credStore) Get(id string) map[string]string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := map[string]string{}
	for k, v := range c.creds[id] {
		out[k] = v
	}
	return out
}

func (c *credStore) Keys(id string) []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	var keys []string
	for k, v := range c.creds[id] {
		if v != "" {
			keys = append(keys, k)
		}
	}
	return keys
}

func (c *credStore) Delete(id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.creds, id)
	return c.saveLocked()
}

func parseExtraEnv(block string) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(block, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		out[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}
	return out
}

func envMapForIssue(creds map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range creds {
		if k == "EXTRA_ENV" {
			for ek, ev := range parseExtraEnv(v) {
				out[ek] = ev
			}
			continue
		}
		if k == "LEGO_PROVIDER_NAME" {
			continue
		}
		out[k] = v
	}
	return out
}

func providerName(configured string, creds map[string]string) string {
	if configured != "" && configured != "other" {
		return configured
	}
	if name := strings.TrimSpace(creds["LEGO_PROVIDER_NAME"]); name != "" {
		return name
	}
	return configured
}

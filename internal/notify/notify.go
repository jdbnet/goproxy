package notify

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"git.jdbnet.co.uk/jamie/goproxy/internal/proxyconfig"
)

type Event struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	Body     string `json:"body"`
	Severity string `json:"severity"`
}

type Notifier struct {
	mu       sync.RWMutex
	webhooks []proxyconfig.Webhook
	client   *http.Client
}

func New() *Notifier {
	return &Notifier{client: &http.Client{Timeout: 10 * time.Second}}
}

func (n *Notifier) Reload(cfg *proxyconfig.Config) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.webhooks = append([]proxyconfig.Webhook(nil), cfg.Notifications.Webhooks...)
}

func (n *Notifier) Send(ev Event) {
	n.mu.RLock()
	hooks := append([]proxyconfig.Webhook(nil), n.webhooks...)
	n.mu.RUnlock()
	for _, h := range hooks {
		if !matchTrigger(h.Triggers, ev.Type) {
			continue
		}
		go n.post(h, ev)
	}
}

func matchTrigger(triggers []string, typ string) bool {
	if len(triggers) == 0 {
		return true
	}
	for _, t := range triggers {
		if t == typ || t == "*" {
			return true
		}
		if strings.HasSuffix(t, ".*") && strings.HasPrefix(typ, strings.TrimSuffix(t, "*")) {
			return true
		}
	}
	return false
}

func (n *Notifier) post(h proxyconfig.Webhook, ev Event) {
	var body []byte
	var err error
	switch h.Format {
	case "discord":
		body, err = json.Marshal(map[string]any{
			"embeds": []map[string]any{{
				"title":       ev.Title,
				"description": ev.Body,
				"color":       discordColor(ev.Severity),
			}},
		})
	case "slack":
		body, err = json.Marshal(map[string]any{
			"text": ev.Title + "\n" + ev.Body,
		})
	default:
		body, err = json.Marshal(ev)
	}
	if err != nil {
		return
	}
	req, err := http.NewRequest(http.MethodPost, h.URL, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := n.client.Do(req)
	if err != nil {
		slog.Error("webhook failed", "id", h.ID, "err", err)
		return
	}
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		slog.Error("webhook status", "id", h.ID, "status", resp.StatusCode)
	}
}

func discordColor(sev string) int {
	switch sev {
	case "error":
		return 0xE74C3C
	case "warn":
		return 0xF1C40F
	default:
		return 0x1EBE8A
	}
}

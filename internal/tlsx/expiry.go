package tlsx

import (
	"fmt"
	"sort"
	"time"

	"git.jdbnet.co.uk/jamie/goproxy/internal/notify"
	"git.jdbnet.co.uk/jamie/goproxy/internal/proxyconfig"
)

type expiryAlertState struct {
	level int
	at    time.Time
}

// expiryAlertLevel returns the alert bucket for daysLeft: warnBefore, 7, 3, 1, or 0 (expired).
// Returns -1 when no alert is needed.
func expiryAlertLevel(daysLeft, warnBeforeDays int) int {
	if daysLeft < 0 {
		return 0
	}
	thresholds := dedupeThresholds(warnBeforeDays, 7, 3, 1)
	sort.Ints(thresholds)
	for _, t := range thresholds {
		if daysLeft <= t {
			return t
		}
	}
	return -1
}

func dedupeThresholds(values ...int) []int {
	seen := map[int]bool{}
	var out []int
	for _, v := range values {
		if v <= 0 || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(out)))
	return out
}

func shouldNotifyExpiry(prev expiryAlertState, ok bool, level int, now time.Time) bool {
	if level < 0 {
		return false
	}
	if !ok {
		return true
	}
	if level == 0 && prev.level == 0 {
		return now.Sub(prev.at) >= 24*time.Hour
	}
	return level < prev.level
}

func (s *Store) warnBeforeDays() int {
	days := int(s.app.TLS.WarnBefore.Duration.Hours() / 24)
	if days <= 0 {
		return 14
	}
	return days
}

func (s *Store) checkExpiryWarnings() {
	s.mu.RLock()
	cfg := s.cfg
	s.mu.RUnlock()
	if cfg == nil {
		return
	}
	warnBefore := s.warnBeforeDays()
	now := time.Now()
	for _, c := range cfg.Certificates {
		info := s.Expiry(c.ID, c.CertFile)
		if info.DaysLeft == nil {
			s.resetExpiryAlert(c.ID)
			continue
		}
		days := *info.DaysLeft
		level := expiryAlertLevel(days, warnBefore)
		if s.metrics != nil && info.ExpiresAt != "" {
			if t, err := time.Parse(time.RFC3339, info.ExpiresAt); err == nil {
				s.metrics.SetCertExpiry(c.ID, time.Until(t).Seconds())
			}
		}
		if level < 0 {
			s.resetExpiryAlert(c.ID)
			continue
		}
		prev, ok := s.expiryAlertState(c.ID)
		if !shouldNotifyExpiry(prev, ok, level, now) {
			continue
		}
		s.sendExpiryAlert(c, info, level)
		s.setExpiryAlert(c.ID, level, now)
	}
}

func (s *Store) expiryAlertState(id string) (expiryAlertState, bool) {
	s.expiryMu.Lock()
	defer s.expiryMu.Unlock()
	st, ok := s.expiryAlert[id]
	return st, ok
}

func (s *Store) setExpiryAlert(id string, level int, at time.Time) {
	s.expiryMu.Lock()
	s.expiryAlert[id] = expiryAlertState{level: level, at: at}
	s.expiryMu.Unlock()
}

func (s *Store) resetExpiryAlert(id string) {
	s.expiryMu.Lock()
	delete(s.expiryAlert, id)
	s.expiryMu.Unlock()
}

func (s *Store) sendExpiryAlert(c proxyconfig.Certificate, info ExpiryInfo, level int) {
	if s.notify == nil {
		return
	}
	name := c.Name
	if name == "" && len(c.Domains) > 0 {
		name = c.Domains[0]
	}
	if name == "" {
		name = c.ID
	}
	days := *info.DaysLeft
	var body string
	if level == 0 {
		if days < 0 {
			body = fmt.Sprintf("Certificate %s expired %d day(s) ago (not after %s)", name, -days, info.ExpiresAt)
		} else {
			body = fmt.Sprintf("Certificate %s has expired (not after %s)", name, info.ExpiresAt)
		}
		s.notify.Send(notify.Event{
			Type:     "cert.expiry.expired",
			Title:    "Certificate expired",
			Body:     body,
			Severity: "error",
		})
		return
	}
	body = fmt.Sprintf("Certificate %s expires in %d day(s) (not after %s)", name, days, info.ExpiresAt)
	s.notify.Send(notify.Event{
		Type:     "cert.expiry.warning",
		Title:    "Certificate expiring soon",
		Body:     body,
		Severity: "warn",
	})
}

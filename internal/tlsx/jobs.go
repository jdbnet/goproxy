package tlsx

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

type JobLine struct {
	At      string `json:"at"`
	Message string `json:"message"`
}

type Job struct {
	ID     string    `json:"id"`
	Action string    `json:"action"`
	Status string    `json:"status"`
	Error  string    `json:"error,omitempty"`
	Lines  []JobLine `json:"lines"`
}

type jobBook struct {
	mu   sync.Mutex
	byID map[string]*Job
}

func newJobBook() *jobBook {
	return &jobBook{byID: map[string]*Job{}}
}

func (b *jobBook) Begin(id, action string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.byID[id] = &Job{ID: id, Action: action, Status: "running"}
}

func (b *jobBook) Append(id, msg string) {
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	j := b.byID[id]
	if j == nil {
		j = &Job{ID: id, Action: "issue", Status: "running"}
		b.byID[id] = j
	}
	j.Lines = append(j.Lines, JobLine{At: time.Now().UTC().Format(time.RFC3339), Message: msg})
	if len(j.Lines) > 400 {
		j.Lines = j.Lines[len(j.Lines)-400:]
	}
}

func (b *jobBook) Finish(id string, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	j := b.byID[id]
	if j == nil {
		j = &Job{ID: id, Action: "issue"}
		b.byID[id] = j
	}
	if err != nil {
		j.Status = "error"
		j.Error = err.Error()
		j.Lines = append(j.Lines, JobLine{At: time.Now().UTC().Format(time.RFC3339), Message: "error: " + err.Error()})
		return
	}
	j.Status = "success"
	j.Lines = append(j.Lines, JobLine{At: time.Now().UTC().Format(time.RFC3339), Message: "done"})
}

func (b *jobBook) Get(id string) *Job {
	b.mu.Lock()
	defer b.mu.Unlock()
	j := b.byID[id]
	if j == nil {
		return nil
	}
	cp := *j
	cp.Lines = append([]JobLine(nil), j.Lines...)
	return &cp
}

type jobLegoLogger struct {
	append func(string)
}

func (l jobLegoLogger) Fatal(args ...any)   { l.append(fmt.Sprint(args...)) }
func (l jobLegoLogger) Fatalln(args ...any) { l.append(fmt.Sprintln(args...)) }
func (l jobLegoLogger) Fatalf(format string, args ...any) {
	l.append(fmt.Sprintf(format, args...))
}
func (l jobLegoLogger) Print(args ...any)   { l.append(fmt.Sprint(args...)) }
func (l jobLegoLogger) Println(args ...any) { l.append(fmt.Sprintln(args...)) }
func (l jobLegoLogger) Printf(format string, args ...any) {
	l.append(fmt.Sprintf(format, args...))
}

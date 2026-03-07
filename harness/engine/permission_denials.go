package engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	PermissionDecisionBlocked            = "blocked"
	PermissionDecisionNeedsApproval      = "needs_approval"
	PermissionDecisionAllowedAfterReview = "allowed_after_review"
)

type PermissionDenial struct {
	Version    int      `json:"version"`
	Timestamp  string   `json:"timestamp"`
	Agent      string   `json:"agent"`
	Command    string   `json:"command"`
	Args       []string `json:"args,omitempty"`
	Cwd        string   `json:"cwd,omitempty"`
	Decision   string   `json:"decision"`
	Reason     string   `json:"reason"`
	PolicyRule string   `json:"policy_rule,omitempty"`
}

type PermissionDenialLog struct {
	path string
	now  func() time.Time
	mu   sync.Mutex
}

func NewPermissionDenialLog(projectPath string) *PermissionDenialLog {
	return &PermissionDenialLog{
		path: filepath.Join(projectPath, ".doombox", "permission-denials.jsonl"),
		now:  time.Now,
	}
}

func NewPermissionDenialLogAtPath(path string) *PermissionDenialLog {
	return &PermissionDenialLog{
		path: path,
		now:  time.Now,
	}
}

func (l *PermissionDenialLog) SetNow(nowFn func() time.Time) {
	if nowFn == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.now = nowFn
}

func (l *PermissionDenialLog) Write(d PermissionDenial) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.path == "" {
		return errors.New("permission-denials path is required")
	}

	d.Version = 1
	if d.Timestamp == "" {
		d.Timestamp = l.now().UTC().Format(time.RFC3339)
	}
	d.Agent = normalizeAgent(d.Agent)
	if !allowedPermissionDecision[d.Decision] {
		return fmt.Errorf("unsupported decision %q", d.Decision)
	}
	if d.Command == "" {
		return errors.New("command is required")
	}
	if d.Reason == "" {
		return errors.New("reason is required")
	}

	f, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	b, err := json.Marshal(d)
	if err != nil {
		return err
	}
	_, err = f.Write(append(b, '\n'))
	return err
}

var allowedPermissionDecision = map[string]bool{
	PermissionDecisionBlocked:            true,
	PermissionDecisionNeedsApproval:      true,
	PermissionDecisionAllowedAfterReview: true,
}

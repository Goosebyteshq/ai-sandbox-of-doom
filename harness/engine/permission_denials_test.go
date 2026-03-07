package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPermissionDenialLogWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "permission-denials.jsonl")
	log := NewPermissionDenialLogAtPath(path)
	log.SetNow(func() time.Time {
		return time.Date(2026, 3, 6, 21, 0, 0, 0, time.UTC)
	})

	err := log.Write(PermissionDenial{
		Agent:    "codex",
		Command:  "docker",
		Args:     []string{"build", "."},
		Cwd:      "/workspace/project",
		Decision: PermissionDecisionNeedsApproval,
		Reason:   "requires host docker socket permission",
	})
	if err != nil {
		t.Fatalf("write denial: %v", err)
	}

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read denials file: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}

	var d PermissionDenial
	if err := json.Unmarshal([]byte(lines[0]), &d); err != nil {
		t.Fatalf("unmarshal denial: %v", err)
	}
	if d.Timestamp != "2026-03-06T21:00:00Z" {
		t.Fatalf("unexpected timestamp: %s", d.Timestamp)
	}
	if d.Version != 1 {
		t.Fatalf("unexpected version: %d", d.Version)
	}
	if d.Agent != "codex" {
		t.Fatalf("unexpected agent: %s", d.Agent)
	}
}

func TestPermissionDenialLogValidation(t *testing.T) {
	log := NewPermissionDenialLogAtPath(filepath.Join(t.TempDir(), "permission-denials.jsonl"))

	if err := log.Write(PermissionDenial{
		Agent:    "codex",
		Command:  "",
		Decision: PermissionDecisionBlocked,
		Reason:   "missing command",
	}); err == nil {
		t.Fatal("expected error for missing command")
	}

	if err := log.Write(PermissionDenial{
		Agent:    "codex",
		Command:  "rm",
		Decision: "maybe",
		Reason:   "bad decision",
	}); err == nil {
		t.Fatal("expected error for invalid decision")
	}
}

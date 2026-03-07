package harness

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Goosebyteshq/doombox/harness/engine"
)

func TestEnsureHarnessFilesCreatesDefaults(t *testing.T) {
	projectDir := t.TempDir()
	now := time.Date(2026, 3, 6, 12, 0, 0, 0, time.UTC)
	cfg, err := ensureHarnessFiles(projectDir, "codex", now)
	if err != nil {
		t.Fatalf("ensureHarnessFiles failed: %v", err)
	}
	if cfg.Provider != "codex" {
		t.Fatalf("expected provider codex, got %q", cfg.Provider)
	}

	for _, p := range []string{
		filepath.Join(projectDir, ".doombox", "harness.json"),
		filepath.Join(projectDir, ".doombox", "policy.json"),
		filepath.Join(projectDir, ".doombox", "todo.json"),
		filepath.Join(projectDir, ".doombox", "session-log.jsonl"),
		filepath.Join(projectDir, ".doombox", "events.jsonl"),
		filepath.Join(projectDir, ".doombox", "permission-denials.jsonl"),
	} {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("expected file %s: %v", p, err)
		}
	}

	policyPath := filepath.Join(projectDir, ".doombox", "policy.json")
	var policy policyFile
	b, err := os.ReadFile(policyPath)
	if err != nil {
		t.Fatalf("read policy: %v", err)
	}
	if err := json.Unmarshal(b, &policy); err != nil {
		t.Fatalf("unmarshal policy: %v", err)
	}
	if policy.CheckpointEveryActions != 4 {
		t.Fatalf("expected checkpoint_every_actions=4, got %d", policy.CheckpointEveryActions)
	}
	if policy.Canary.Percent != 0 {
		t.Fatalf("expected canary.percent=0, got %d", policy.Canary.Percent)
	}
}

func TestEnsureAdversarialTodoAddsSingleOpenItem(t *testing.T) {
	projectDir := t.TempDir()
	now := time.Date(2026, 3, 6, 12, 0, 0, 0, time.UTC)
	cfg, err := ensureHarnessFiles(projectDir, "codex", now)
	if err != nil {
		t.Fatalf("ensureHarnessFiles failed: %v", err)
	}
	bus := engine.NewBus(projectDir)

	if err := ensureAdversarialTodo(projectDir, "codex", cfg, now, "test", bus); err != nil {
		t.Fatalf("ensureAdversarialTodo failed: %v", err)
	}
	if err := ensureAdversarialTodo(projectDir, "codex", cfg, now.Add(20*time.Minute), "test", bus); err != nil {
		t.Fatalf("ensureAdversarialTodo failed: %v", err)
	}

	todoPath := filepath.Join(projectDir, ".doombox", "todo.json")
	b, err := os.ReadFile(todoPath)
	if err != nil {
		t.Fatalf("read todo: %v", err)
	}
	var todos todoFile
	if err := json.Unmarshal(b, &todos); err != nil {
		t.Fatalf("unmarshal todo: %v", err)
	}

	open := 0
	for _, item := range todos.Items {
		if item.Type == "adversarial_check" && item.Status == "open" {
			open++
		}
	}
	if open != 1 {
		t.Fatalf("expected exactly one open adversarial_check item, got %d", open)
	}

	eventsPath := filepath.Join(projectDir, ".doombox", "events.jsonl")
	events, err := os.ReadFile(eventsPath)
	if err != nil {
		t.Fatalf("read events: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(events)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected checkpoint_due + checkpoint_written events, got %d lines", len(lines))
	}
	var due engine.Event
	if err := json.Unmarshal([]byte(lines[0]), &due); err != nil {
		t.Fatalf("unmarshal event: %v", err)
	}
	if due.EventType != engine.EventTypeCheckpointDue {
		t.Fatalf("expected event_type checkpoint_due, got %q", due.EventType)
	}

	var written engine.Event
	if err := json.Unmarshal([]byte(lines[1]), &written); err != nil {
		t.Fatalf("unmarshal event: %v", err)
	}
	if written.EventType != engine.EventTypeCheckpointWrite {
		t.Fatalf("expected event_type checkpoint_written, got %q", written.EventType)
	}

	checkpointDir := filepath.Join(projectDir, ".doombox", "checkpoints")
	files, err := os.ReadDir(checkpointDir)
	if err != nil {
		t.Fatalf("read checkpoint dir: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected exactly one checkpoint file, got %d", len(files))
	}
}

func TestRunWithSessionWritesSessionEventsToBus(t *testing.T) {
	projectDir := t.TempDir()
	err := RunWithSession("codex", projectDir, os.Stdout, func() error { return nil })
	if err != nil {
		t.Fatalf("RunWithSession failed: %v", err)
	}

	eventsPath := filepath.Join(projectDir, ".doombox", "events.jsonl")
	events, err := os.ReadFile(eventsPath)
	if err != nil {
		t.Fatalf("read events: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(events)), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 events, got %d", len(lines))
	}

	var first engine.Event
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatalf("unmarshal first event: %v", err)
	}
	if first.EventType != engine.EventTypeSessionStart {
		t.Fatalf("expected first event session_start, got %q", first.EventType)
	}

	var last engine.Event
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &last); err != nil {
		t.Fatalf("unmarshal last event: %v", err)
	}
	if last.EventType != engine.EventTypeSessionEnd {
		t.Fatalf("expected last event session_end, got %q", last.EventType)
	}
}

func TestInitializeCreatesHarnessFiles(t *testing.T) {
	projectDir := t.TempDir()
	if err := Initialize("codex", projectDir); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	required := []string{
		filepath.Join(projectDir, ".doombox", "harness.json"),
		filepath.Join(projectDir, ".doombox", "policy.json"),
		filepath.Join(projectDir, ".doombox", "todo.json"),
		filepath.Join(projectDir, ".doombox", "session-log.jsonl"),
		filepath.Join(projectDir, ".doombox", "events.jsonl"),
		filepath.Join(projectDir, ".doombox", "permission-denials.jsonl"),
	}
	for _, path := range required {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected file %s: %v", path, err)
		}
	}
}

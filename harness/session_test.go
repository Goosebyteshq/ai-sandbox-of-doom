package harness

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
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
		filepath.Join(projectDir, ".doombox", "todo.json"),
		filepath.Join(projectDir, ".doombox", "session-log.jsonl"),
	} {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("expected file %s: %v", p, err)
		}
	}
}

func TestEnsureAdversarialTodoAddsSingleOpenItem(t *testing.T) {
	projectDir := t.TempDir()
	now := time.Date(2026, 3, 6, 12, 0, 0, 0, time.UTC)
	cfg, err := ensureHarnessFiles(projectDir, "codex", now)
	if err != nil {
		t.Fatalf("ensureHarnessFiles failed: %v", err)
	}

	if err := ensureAdversarialTodo(projectDir, "codex", cfg, now, "test"); err != nil {
		t.Fatalf("ensureAdversarialTodo failed: %v", err)
	}
	if err := ensureAdversarialTodo(projectDir, "codex", cfg, now.Add(20*time.Minute), "test"); err != nil {
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
}

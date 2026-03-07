package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCheckpointStoreWrite(t *testing.T) {
	projectDir := t.TempDir()
	store := NewCheckpointStore(projectDir)
	store.SetNow(func() time.Time {
		return time.Date(2026, 3, 6, 20, 0, 0, 0, time.UTC)
	})

	cp, path, err := store.Write(CheckpointInput{
		ID:              "cp-review-001",
		Agent:           "codex",
		CurrentGoal:     "Stabilize checkpoint trigger behavior",
		FilesChanged:    []string{"harness/engine/checkpoint_trigger.go"},
		OutOfScopeFiles: []string{},
		NextStepToScope: "Implement and test immediate checkpoint triggers.",
		NonObviousFileJustifications: []CheckpointJustification{
			{
				File: "harness/engine/checkpoint_trigger.go",
				Why:  "Core trigger implementation for event-driven supervisor.",
			},
		},
		TestsRun: []CheckpointTestRun{
			{Cmd: "go test ./...", Result: "pass"},
		},
		RiskLevel: "medium",
	})
	if err != nil {
		t.Fatalf("Write checkpoint: %v", err)
	}

	if cp.ID != "cp-review-001" {
		t.Fatalf("unexpected checkpoint id: %s", cp.ID)
	}
	if cp.Timestamp != "2026-03-06T20:00:00Z" {
		t.Fatalf("unexpected checkpoint timestamp: %s", cp.Timestamp)
	}
	if cp.Agent != "codex" {
		t.Fatalf("unexpected checkpoint agent: %s", cp.Agent)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected checkpoint file at %s: %v", path, err)
	}

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read checkpoint file: %v", err)
	}
	var persisted Checkpoint
	if err := json.Unmarshal(b, &persisted); err != nil {
		t.Fatalf("unmarshal checkpoint: %v", err)
	}
	if persisted.CurrentGoal == "" || persisted.NextStepToScope == "" {
		t.Fatal("expected persisted checkpoint required fields")
	}
}

func TestCheckpointStoreWriteRequiresRequiredFields(t *testing.T) {
	store := NewCheckpointStore(t.TempDir())
	if _, _, err := store.Write(CheckpointInput{
		Agent:           "codex",
		CurrentGoal:     "",
		NextStepToScope: "something",
	}); err == nil {
		t.Fatal("expected error when current_goal is missing")
	}
	if _, _, err := store.Write(CheckpointInput{
		Agent:           "codex",
		CurrentGoal:     "goal",
		NextStepToScope: "",
	}); err == nil {
		t.Fatal("expected error when next_step_to_scope is missing")
	}
}

func TestCheckpointStoreWritesUnderDoomboxCheckpoints(t *testing.T) {
	projectDir := t.TempDir()
	store := NewCheckpointStore(projectDir)

	_, path, err := store.Write(CheckpointInput{
		Agent:           "codex",
		CurrentGoal:     "Goal",
		NextStepToScope: "Next",
	})
	if err != nil {
		t.Fatalf("Write checkpoint: %v", err)
	}

	wantPrefix := filepath.Join(projectDir, ".doombox", "checkpoints") + string(filepath.Separator)
	if len(path) <= len(wantPrefix) || path[:len(wantPrefix)] != wantPrefix {
		t.Fatalf("expected checkpoint path under %s, got %s", wantPrefix, path)
	}
}

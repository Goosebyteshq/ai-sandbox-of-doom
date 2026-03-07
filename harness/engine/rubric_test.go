package engine

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScoreTrajectory(t *testing.T) {
	events := []Event{
		{EventType: EventTypeToolInvocation, RiskClassification: "safe", Payload: map[string]any{"command": "go test ./..."}},
		{EventType: EventTypeTestResult, Payload: map[string]any{"result": "pass"}},
		{EventType: EventTypeToolInvocation, RiskClassification: "justify", Payload: map[string]any{"command": "git push --force"}},
		{EventType: EventTypeTestResult, Payload: map[string]any{"result": "fail"}},
	}
	checkpoints := []Checkpoint{
		{
			FilesChanged:    []string{"a.go", "b.go"},
			OutOfScopeFiles: []string{"b.go"},
		},
	}

	rubric := ScoreTrajectory(events, checkpoints)
	if rubric.Score <= 0 || rubric.Score > 1 {
		t.Fatalf("unexpected score: %v", rubric.Score)
	}
	if rubric.ScopeDiscipline != 0.5 {
		t.Fatalf("unexpected scope discipline: %v", rubric.ScopeDiscipline)
	}
	if rubric.TestDiscipline != 0.5 {
		t.Fatalf("unexpected test discipline: %v", rubric.TestDiscipline)
	}
}

func TestLoadEventsAndCheckpoints(t *testing.T) {
	dir := t.TempDir()
	eventsPath := filepath.Join(dir, "events.jsonl")
	err := os.WriteFile(eventsPath, []byte("{\"version\":1,\"timestamp\":\"2026-03-07T00:00:00Z\",\"event_type\":\"session_start\",\"source\":\"system\"}\n"), 0644)
	if err != nil {
		t.Fatalf("write events: %v", err)
	}
	events, err := LoadEventsJSONL(eventsPath)
	if err != nil {
		t.Fatalf("load events: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	cpDir := filepath.Join(dir, "checkpoints")
	if err := os.MkdirAll(cpDir, 0755); err != nil {
		t.Fatalf("mkdir checkpoints: %v", err)
	}
	err = os.WriteFile(filepath.Join(cpDir, "cp-1.json"), []byte("{\"version\":1,\"id\":\"cp-1\",\"timestamp\":\"2026-03-07T00:00:00Z\",\"agent\":\"codex\",\"current_goal\":\"g\",\"files_changed\":[],\"out_of_scope_files\":[],\"next_step_to_scope\":\"n\",\"non_obvious_file_justifications\":[]}"), 0644)
	if err != nil {
		t.Fatalf("write checkpoint: %v", err)
	}
	checkpoints, err := LoadCheckpoints(cpDir)
	if err != nil {
		t.Fatalf("load checkpoints: %v", err)
	}
	if len(checkpoints) != 1 {
		t.Fatalf("expected 1 checkpoint, got %d", len(checkpoints))
	}
}

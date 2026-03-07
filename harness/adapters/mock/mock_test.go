package mock

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRunnerRun(t *testing.T) {
	scenario := Scenario{
		Version: 1,
		Name:    "basic",
		Agent:   "codex",
		Actions: []Action{
			{Kind: "edit", Message: "edited foo", Files: []string{"foo.go"}},
			{Kind: "test", Message: "tests pass", Result: "pass"},
			{Kind: "deny", Message: "blocked rm", Command: "rm -rf /", Risk: "block"},
		},
	}

	tick := 0
	r := Runner{
		Now: func() time.Time {
			base := time.Date(2026, 3, 6, 10, 0, 0, 0, time.UTC)
			ts := base.Add(time.Duration(tick) * time.Second)
			tick++
			return ts
		},
	}

	var got []Event
	if err := r.Run(scenario, func(e Event) error {
		got = append(got, e)
		return nil
	}); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	if len(got) != 5 {
		t.Fatalf("expected 5 events, got %d", len(got))
	}
	if got[0].EventType != "session_start" {
		t.Fatalf("first event should be session_start, got %q", got[0].EventType)
	}
	if got[1].EventType != "edit_cluster" {
		t.Fatalf("second event should be edit_cluster, got %q", got[1].EventType)
	}
	if got[2].EventType != "test_result" {
		t.Fatalf("third event should be test_result, got %q", got[2].EventType)
	}
	if got[3].EventType != "permission_denied" {
		t.Fatalf("fourth event should be permission_denied, got %q", got[3].EventType)
	}
	if got[4].EventType != "session_end" {
		t.Fatalf("last event should be session_end, got %q", got[4].EventType)
	}
}

func TestLoadScenario(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "scenario.json")
	raw := `{
	  "version": 1,
	  "name": "fixture",
	  "agent": "codex",
	  "actions": [
	    { "kind": "tool", "command": "go test ./...", "message": "run tests" }
	  ]
	}`
	if err := os.WriteFile(path, []byte(raw), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	s, err := LoadScenario(path)
	if err != nil {
		t.Fatalf("load scenario: %v", err)
	}
	if s.Name != "fixture" {
		t.Fatalf("unexpected name: %q", s.Name)
	}
	if len(s.Actions) != 1 {
		t.Fatalf("unexpected actions length: %d", len(s.Actions))
	}
}

func TestEventsMarshal(t *testing.T) {
	e := Event{
		Version:   1,
		ID:        "x",
		Timestamp: "2026-03-06T10:00:00Z",
		EventType: "tool_invocation",
		Source:    "agent",
		Agent:     "codex",
	}
	if _, err := json.Marshal(e); err != nil {
		t.Fatalf("marshal event: %v", err)
	}
}

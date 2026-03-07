package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBusEmitValidatesInput(t *testing.T) {
	bus := NewBusAtPath(filepath.Join(t.TempDir(), "events.jsonl"))
	bus.SetNow(func() time.Time {
		return time.Date(2026, 3, 6, 12, 34, 56, 0, time.UTC)
	})

	if err := bus.Emit(Event{
		EventType: EventTypeToolInvocation,
		Source:    SourceAgent,
		Agent:     "codex",
		Message:   "run go test",
	}); err != nil {
		t.Fatalf("emit valid event: %v", err)
	}

	events := readEvents(t, bus.eventsPath)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Version != 1 {
		t.Fatalf("expected version 1, got %d", events[0].Version)
	}
	if events[0].Timestamp != "2026-03-06T12:34:56Z" {
		t.Fatalf("unexpected timestamp: %s", events[0].Timestamp)
	}

	if err := bus.Emit(Event{
		EventType: "bad_event_type",
		Source:    SourceAgent,
	}); err == nil {
		t.Fatal("expected invalid event type error")
	}

	if err := bus.Emit(Event{
		EventType:          EventTypeToolInvocation,
		Source:             SourceAgent,
		RiskClassification: "very-risky",
	}); err == nil {
		t.Fatal("expected invalid risk classification error")
	}
}

func TestBusTypedEmitters(t *testing.T) {
	eventsPath := filepath.Join(t.TempDir(), "events.jsonl")
	bus := NewBusAtPath(eventsPath)
	bus.SetNow(func() time.Time {
		return time.Date(2026, 3, 6, 18, 0, 0, 0, time.UTC)
	})

	if err := bus.EmitSessionStart("codex", "session start"); err != nil {
		t.Fatalf("EmitSessionStart: %v", err)
	}
	if err := bus.EmitToolInvocation("codex", "go test ./...", "run tests", "safe", nil); err != nil {
		t.Fatalf("EmitToolInvocation: %v", err)
	}
	if err := bus.EmitEditCluster("codex", "edit files", "justify", []string{"README.md"}, nil); err != nil {
		t.Fatalf("EmitEditCluster: %v", err)
	}
	if err := bus.EmitTestResult("codex", "go test ./...", "pass", "tests green", nil); err != nil {
		t.Fatalf("EmitTestResult: %v", err)
	}
	if err := bus.EmitGateDecision("codex", "pre-commit", "pass", "gate passed", "safe", nil); err != nil {
		t.Fatalf("EmitGateDecision: %v", err)
	}
	if err := bus.EmitCheckpointWritten("codex", "cp-123", ".doombox/checkpoints/cp-123.json", "checkpoint persisted"); err != nil {
		t.Fatalf("EmitCheckpointWritten: %v", err)
	}
	if err := bus.EmitSessionEnd("codex", "session end"); err != nil {
		t.Fatalf("EmitSessionEnd: %v", err)
	}

	events := readEvents(t, eventsPath)
	if len(events) != 7 {
		t.Fatalf("expected 7 events, got %d", len(events))
	}

	wantTypes := []string{
		EventTypeSessionStart,
		EventTypeToolInvocation,
		EventTypeEditCluster,
		EventTypeTestResult,
		EventTypeGateDecision,
		EventTypeCheckpointWrite,
		EventTypeSessionEnd,
	}
	for i, want := range wantTypes {
		if events[i].EventType != want {
			t.Fatalf("event %d type mismatch: want %q got %q", i, want, events[i].EventType)
		}
	}

	command, _ := events[1].Payload["command"].(string)
	if command != "go test ./..." {
		t.Fatalf("unexpected tool command payload: %q", command)
	}

	result, _ := events[3].Payload["result"].(string)
	if result != "pass" {
		t.Fatalf("unexpected test result payload: %q", result)
	}

	gate, _ := events[4].Payload["gate"].(string)
	if gate != "pre-commit" {
		t.Fatalf("unexpected gate payload: %q", gate)
	}

	checkpointID, _ := events[5].Payload["checkpoint_id"].(string)
	if checkpointID != "cp-123" {
		t.Fatalf("unexpected checkpoint_id payload: %q", checkpointID)
	}
}

func TestBusEmitClassifiedToolInvocation(t *testing.T) {
	dir := t.TempDir()
	eventsPath := filepath.Join(dir, "events.jsonl")
	policyPath := PolicyPathFromEventsPath(eventsPath)
	err := os.WriteFile(policyPath, []byte(`{
  "blocked_command_prefixes": ["my-blocked-command"]
}`), 0644)
	if err != nil {
		t.Fatalf("write policy file: %v", err)
	}

	bus := NewBusAtPath(eventsPath)

	err = bus.EmitClassifiedToolInvocation("codex", ToolInvocation{
		Command: "my-blocked-command now",
		Args:    []string{"my-blocked-command", "now"},
		Cwd:     "/workspace/project",
		Files:   []string{"README.md"},
	}, "blocked command", nil)
	if err != nil {
		t.Fatalf("EmitClassifiedToolInvocation: %v", err)
	}

	events := readEvents(t, eventsPath)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].RiskClassification != "block" {
		t.Fatalf("expected block risk, got %q", events[0].RiskClassification)
	}
	rule, _ := events[0].Payload["tool_classification_rule"].(string)
	if rule == "" {
		t.Fatal("expected tool_classification_rule payload to be set")
	}
}

func readEvents(t *testing.T, path string) []Event {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read events: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	out := make([]Event, 0, len(lines))
	for _, line := range lines {
		var ev Event
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			t.Fatalf("unmarshal event line %q: %v", line, err)
		}
		out = append(out, ev)
	}
	return out
}

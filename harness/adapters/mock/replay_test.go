package mock

import (
	"path/filepath"
	"testing"
	"time"
)

func TestReplayEventsJSONLFromFixture(t *testing.T) {
	path := fixturePath(t, "checkpoint-gate-flow.json")
	scenario, err := LoadScenario(path)
	if err != nil {
		t.Fatalf("load scenario: %v", err)
	}

	tick := 0
	r := Runner{
		Now: func() time.Time {
			base := time.Date(2026, 3, 6, 13, 0, 0, 0, time.UTC)
			ts := base.Add(time.Duration(tick) * time.Second)
			tick++
			return ts
		},
	}

	var events []Event
	if err := r.Run(scenario, func(e Event) error {
		events = append(events, e)
		return nil
	}); err != nil {
		t.Fatalf("run scenario: %v", err)
	}

	eventsPath := filepath.Join(t.TempDir(), "events.jsonl")
	if err := WriteEventsJSONL(eventsPath, events); err != nil {
		t.Fatalf("write events: %v", err)
	}

	summary, err := ReplayEventsJSONL(eventsPath)
	if err != nil {
		t.Fatalf("replay events: %v", err)
	}
	if err := RequireEvents(summary); err != nil {
		t.Fatalf("expected non-empty replay: %v", err)
	}

	if summary.EventTypeCounts["checkpoint_due"] != 1 {
		t.Fatalf("expected 1 checkpoint_due event, got %d", summary.EventTypeCounts["checkpoint_due"])
	}
	if summary.EventTypeCounts["gate_decision"] != 2 {
		t.Fatalf("expected 2 gate_decision events, got %d", summary.EventTypeCounts["gate_decision"])
	}
	if summary.OpenTodos != 1 {
		t.Fatalf("expected 1 open todo, got %d", summary.OpenTodos)
	}
	if summary.ClosedTodos != 1 {
		t.Fatalf("expected 1 closed todo, got %d", summary.ClosedTodos)
	}
}

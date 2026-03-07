package engine

import "testing"

func TestCheckpointTriggerEmitsEveryNActionClusters(t *testing.T) {
	trigger := NewCheckpointTrigger(3)

	events := []Event{
		{EventType: EventTypeToolInvocation, Agent: "codex"},
		{EventType: EventTypeEditCluster, Agent: "codex"},
		{EventType: EventTypeTestResult, Agent: "codex"},
		{EventType: EventTypeToolInvocation, Agent: "codex"},
	}

	dueCount := 0
	for i, ev := range events {
		due, ok := trigger.Observe(ev)
		if i == 2 {
			if !ok {
				t.Fatal("expected checkpoint due on third action")
			}
			if due.EventType != EventTypeCheckpointDue {
				t.Fatalf("expected checkpoint_due event type, got %q", due.EventType)
			}
			dueCount++
			continue
		}
		if ok {
			t.Fatalf("did not expect checkpoint due on index %d", i)
		}
	}

	if dueCount != 1 {
		t.Fatalf("expected exactly one checkpoint due event, got %d", dueCount)
	}
	if trigger.TotalActionsSeen() != len(events) {
		t.Fatalf("expected %d actions seen, got %d", len(events), trigger.TotalActionsSeen())
	}
}

func TestCheckpointTriggerIgnoresNonActionEvents(t *testing.T) {
	trigger := NewCheckpointTrigger(2)

	events := []Event{
		{EventType: EventTypeSessionStart, Agent: "codex"},
		{EventType: EventTypeCheckpointDue, Agent: "codex"},
		{EventType: EventTypeGateDecision, Agent: "codex"},
	}
	for _, ev := range events {
		if _, ok := trigger.Observe(ev); ok {
			t.Fatalf("did not expect checkpoint due for event type %q", ev.EventType)
		}
	}

	if trigger.TotalActionsSeen() != 0 {
		t.Fatalf("expected 0 actions seen, got %d", trigger.TotalActionsSeen())
	}
}

func TestCheckpointTriggerDefaultsToFour(t *testing.T) {
	trigger := NewCheckpointTrigger(0)

	for i := 0; i < 3; i++ {
		if _, ok := trigger.Observe(Event{EventType: EventTypeToolInvocation, Agent: "codex"}); ok {
			t.Fatalf("did not expect checkpoint due at action %d", i+1)
		}
	}
	if _, ok := trigger.Observe(Event{EventType: EventTypeToolInvocation, Agent: "codex"}); !ok {
		t.Fatal("expected checkpoint due at action 4")
	}
}

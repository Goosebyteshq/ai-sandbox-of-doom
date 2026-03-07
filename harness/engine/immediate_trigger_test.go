package engine

import "testing"

func TestImmediateCheckpointTriggerTestFail(t *testing.T) {
	trigger := NewImmediateCheckpointTrigger(ImmediateCheckpointPolicy{})

	ev, ok := trigger.Observe(Event{
		EventType: EventTypeTestResult,
		Agent:     "codex",
		Payload: map[string]any{
			"result": "fail",
		},
	})
	if !ok {
		t.Fatal("expected checkpoint due for failing test")
	}
	assertImmediateReason(t, ev, "test_fail")
}

func TestImmediateCheckpointTriggerRiskyTouch(t *testing.T) {
	trigger := NewImmediateCheckpointTrigger(ImmediateCheckpointPolicy{
		RiskyPaths: []string{"infra/", ".github/"},
	})

	ev, ok := trigger.Observe(Event{
		EventType: EventTypeEditCluster,
		Agent:     "codex",
		Payload: map[string]any{
			"files": []any{"README.md", "infra/prod/main.tf"},
		},
	})
	if !ok {
		t.Fatal("expected checkpoint due for risky file touch")
	}
	assertImmediateReason(t, ev, "risky_touch")
}

func TestImmediateCheckpointTriggerLargeDiff(t *testing.T) {
	trigger := NewImmediateCheckpointTrigger(ImmediateCheckpointPolicy{
		LargeDiffLineThreshold: 120,
	})

	ev, ok := trigger.Observe(Event{
		EventType: EventTypeEditCluster,
		Agent:     "codex",
		Payload: map[string]any{
			"diff_lines": 180,
		},
	})
	if !ok {
		t.Fatal("expected checkpoint due for large diff")
	}
	assertImmediateReason(t, ev, "large_diff")
}

func TestImmediateCheckpointTriggerPreCommitAndPrePush(t *testing.T) {
	trigger := NewImmediateCheckpointTrigger(ImmediateCheckpointPolicy{})

	commitEvent, ok := trigger.Observe(Event{
		EventType: EventTypeGateDecision,
		Agent:     "codex",
		Payload: map[string]any{
			"gate": "pre-commit",
		},
	})
	if !ok {
		t.Fatal("expected checkpoint due for pre-commit gate")
	}
	assertImmediateReason(t, commitEvent, "pre_commit")

	pushEvent, ok := trigger.Observe(Event{
		EventType: EventTypeToolInvocation,
		Agent:     "codex",
		Payload: map[string]any{
			"command": "git push origin main",
		},
	})
	if !ok {
		t.Fatal("expected checkpoint due for git push")
	}
	assertImmediateReason(t, pushEvent, "pre_push")
}

func TestImmediateCheckpointTriggerNoMatch(t *testing.T) {
	trigger := NewImmediateCheckpointTrigger(ImmediateCheckpointPolicy{
		RiskyPaths:             []string{"infra/"},
		LargeDiffLineThreshold: 500,
	})

	if _, ok := trigger.Observe(Event{
		EventType: EventTypeTestResult,
		Agent:     "codex",
		Payload: map[string]any{
			"result": "pass",
		},
	}); ok {
		t.Fatal("did not expect checkpoint due for passing test")
	}
}

func assertImmediateReason(t *testing.T, e Event, reason string) {
	t.Helper()
	if e.EventType != EventTypeCheckpointDue {
		t.Fatalf("expected checkpoint_due, got %q", e.EventType)
	}
	gotReason, _ := e.Payload["reason"].(string)
	if gotReason != reason {
		t.Fatalf("expected reason %q, got %q", reason, gotReason)
	}
	trigger, _ := e.Payload["trigger"].(string)
	if trigger != "immediate" {
		t.Fatalf("expected trigger immediate, got %q", trigger)
	}
}

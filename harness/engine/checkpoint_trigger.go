package engine

import (
	"fmt"
	"sync"
)

const defaultCheckpointEveryActions = 4

// CheckpointTrigger emits checkpoint_due after every configured number of
// action-cluster events.
type CheckpointTrigger struct {
	everyActions int
	actionCount  int
	mu           sync.Mutex
}

func NewCheckpointTrigger(everyActions int) *CheckpointTrigger {
	if everyActions <= 0 {
		everyActions = defaultCheckpointEveryActions
	}
	return &CheckpointTrigger{everyActions: everyActions}
}

func (t *CheckpointTrigger) Observe(e Event) (Event, bool) {
	if !isActionClusterEvent(e.EventType) {
		return Event{}, false
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	t.actionCount++
	if t.actionCount%t.everyActions != 0 {
		return Event{}, false
	}

	return Event{
		EventType: EventTypeCheckpointDue,
		Source:    SourceSupervisor,
		Agent:     normalizeAgent(e.Agent),
		Message:   fmt.Sprintf("Checkpoint due after %d action clusters", t.everyActions),
		Payload: map[string]any{
			"trigger":                           "action_cluster",
			"checkpoint_every_actions":          t.everyActions,
			"actions_since_last_action_trigger": t.everyActions,
			"total_actions_seen":                t.actionCount,
			"last_action_event_type":            e.EventType,
		},
	}, true
}

func (t *CheckpointTrigger) TotalActionsSeen() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.actionCount
}

func isActionClusterEvent(eventType string) bool {
	switch eventType {
	case EventTypeToolInvocation, EventTypeEditCluster, EventTypeTestResult:
		return true
	default:
		return false
	}
}

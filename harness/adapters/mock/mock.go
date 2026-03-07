package mock

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"
)

// Event matches the harness event schema shape and can be emitted to events.jsonl.
type Event struct {
	Version            int            `json:"version"`
	ID                 string         `json:"id,omitempty"`
	Timestamp          string         `json:"timestamp"`
	EventType          string         `json:"event_type"`
	Source             string         `json:"source"`
	Agent              string         `json:"agent,omitempty"`
	Message            string         `json:"message,omitempty"`
	RiskClassification string         `json:"risk_classification,omitempty"`
	Payload            map[string]any `json:"payload,omitempty"`
}

// Scenario is a deterministic action script used to test harness behavior
// without a live LLM.
type Scenario struct {
	Version int      `json:"version"`
	Name    string   `json:"name"`
	Agent   string   `json:"agent"`
	Actions []Action `json:"actions"`
}

// Action represents one scripted step in a mock trajectory.
type Action struct {
	ID        string         `json:"id,omitempty"`
	Kind      string         `json:"kind"`
	Message   string         `json:"message,omitempty"`
	Command   string         `json:"command,omitempty"`
	Files     []string       `json:"files,omitempty"`
	Result    string         `json:"result,omitempty"`
	Risk      string         `json:"risk,omitempty"`
	EventType string         `json:"event_type,omitempty"`
	Payload   map[string]any `json:"payload,omitempty"`
}

type Runner struct {
	Now func() time.Time
}

// LoadScenario reads a JSON scenario fixture from disk.
func LoadScenario(path string) (Scenario, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Scenario{}, err
	}
	var s Scenario
	if err := json.Unmarshal(b, &s); err != nil {
		return Scenario{}, err
	}
	if s.Version == 0 {
		s.Version = 1
	}
	if s.Agent == "" {
		s.Agent = "codex"
	}
	return s, nil
}

// Run emits a deterministic event stream from the scenario.
func (r Runner) Run(s Scenario, emit func(Event) error) error {
	if emit == nil {
		return errors.New("emit callback is required")
	}
	nowFn := r.Now
	if nowFn == nil {
		nowFn = time.Now
	}

	start := Event{
		Version:   1,
		ID:        "mock-session-start",
		Timestamp: nowFn().UTC().Format(time.RFC3339),
		EventType: "session_start",
		Source:    "agent",
		Agent:     s.Agent,
		Message:   "mock scenario start: " + s.Name,
	}
	if err := emit(start); err != nil {
		return err
	}

	for i, action := range s.Actions {
		ev, err := actionToEvent(action, s.Agent, nowFn().UTC(), i)
		if err != nil {
			return err
		}
		if err := emit(ev); err != nil {
			return err
		}
	}

	end := Event{
		Version:   1,
		ID:        "mock-session-end",
		Timestamp: nowFn().UTC().Format(time.RFC3339),
		EventType: "session_end",
		Source:    "agent",
		Agent:     s.Agent,
		Message:   "mock scenario end: " + s.Name,
	}
	return emit(end)
}

func actionToEvent(a Action, agent string, ts time.Time, index int) (Event, error) {
	eventType := a.EventType
	switch a.Kind {
	case "edit":
		eventType = "edit_cluster"
	case "tool":
		eventType = "tool_invocation"
	case "test":
		eventType = "test_result"
	case "checkpoint":
		eventType = "checkpoint_due"
	case "gate":
		eventType = "gate_decision"
	case "deny":
		eventType = "permission_denied"
	case "custom":
		if eventType == "" {
			return Event{}, errors.New("custom action requires event_type")
		}
	default:
		return Event{}, fmt.Errorf("unsupported action kind %q", a.Kind)
	}

	payload := map[string]any{}
	for k, v := range a.Payload {
		payload[k] = v
	}
	if a.Command != "" {
		payload["command"] = a.Command
	}
	if len(a.Files) > 0 {
		payload["files"] = a.Files
	}
	if a.Result != "" {
		payload["result"] = a.Result
	}

	id := a.ID
	if id == "" {
		id = fmt.Sprintf("mock-%02d", index+1)
	}
	return Event{
		Version:            1,
		ID:                 id,
		Timestamp:          ts.Format(time.RFC3339),
		EventType:          eventType,
		Source:             "agent",
		Agent:              agent,
		Message:            a.Message,
		RiskClassification: a.Risk,
		Payload:            payload,
	}, nil
}

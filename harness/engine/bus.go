package engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	EventTypeSessionStart    = "session_start"
	EventTypeSessionEnd      = "session_end"
	EventTypeToolInvocation  = "tool_invocation"
	EventTypeEditCluster     = "edit_cluster"
	EventTypeTestResult      = "test_result"
	EventTypeCheckpointDue   = "checkpoint_due"
	EventTypeCheckpointWrite = "checkpoint_written"
	EventTypeGateDecision    = "gate_decision"
	EventTypePermissionDeny  = "permission_denied"
)

const (
	SourceAgent      = "agent"
	SourceSupervisor = "supervisor"
	SourceGate       = "gate"
	SourceHook       = "hook"
	SourceSystem     = "system"
)

// Event matches harness/schemas/event.schema.json.
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

// Bus appends schema-valid JSONL events to .doombox/events.jsonl.
type Bus struct {
	eventsPath string
	now        func() time.Time
	mu         sync.Mutex
}

func NewBus(projectPath string) *Bus {
	return &Bus{
		eventsPath: filepath.Join(projectPath, ".doombox", "events.jsonl"),
		now:        time.Now,
	}
}

func NewBusAtPath(eventsPath string) *Bus {
	return &Bus{
		eventsPath: eventsPath,
		now:        time.Now,
	}
}

func (b *Bus) SetNow(nowFn func() time.Time) {
	if nowFn == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.now = nowFn
}

func (b *Bus) EmitSessionStart(agent, message string) error {
	return b.Emit(Event{
		EventType: EventTypeSessionStart,
		Source:    SourceSystem,
		Agent:     normalizeAgent(agent),
		Message:   message,
	})
}

func (b *Bus) EmitSessionEnd(agent, message string) error {
	return b.Emit(Event{
		EventType: EventTypeSessionEnd,
		Source:    SourceSystem,
		Agent:     normalizeAgent(agent),
		Message:   message,
	})
}

func (b *Bus) EmitToolInvocation(agent, command, message, risk string, payload map[string]any) error {
	withCommand := clonePayload(payload)
	if command != "" {
		if withCommand == nil {
			withCommand = map[string]any{}
		}
		withCommand["command"] = command
	}
	return b.Emit(Event{
		EventType:          EventTypeToolInvocation,
		Source:             SourceAgent,
		Agent:              normalizeAgent(agent),
		Message:            message,
		RiskClassification: risk,
		Payload:            withCommand,
	})
}

func (b *Bus) EmitClassifiedToolInvocation(agent string, invocation ToolInvocation, message string, payload map[string]any) error {
	classification := ToolClassifierFromPolicyFile(PolicyPathFromEventsPath(b.eventsPath)).Classify(invocation)
	withMeta := clonePayload(payload)
	if withMeta == nil {
		withMeta = map[string]any{}
	}
	withMeta["tool_classification_reason"] = classification.Reason
	withMeta["tool_classification_rule"] = classification.Rule
	if invocation.Command != "" {
		withMeta["command"] = invocation.Command
	}
	if len(invocation.Args) > 0 {
		withMeta["args"] = invocation.Args
	}
	if invocation.Cwd != "" {
		withMeta["cwd"] = invocation.Cwd
	}
	if len(invocation.Files) > 0 {
		withMeta["files"] = invocation.Files
	}
	return b.EmitToolInvocation(agent, invocation.Command, message, classification.Risk, withMeta)
}

func (b *Bus) EmitEditCluster(agent, message, risk string, files []string, payload map[string]any) error {
	withFiles := clonePayload(payload)
	if len(files) > 0 {
		if withFiles == nil {
			withFiles = map[string]any{}
		}
		withFiles["files"] = files
	}
	return b.Emit(Event{
		EventType:          EventTypeEditCluster,
		Source:             SourceAgent,
		Agent:              normalizeAgent(agent),
		Message:            message,
		RiskClassification: risk,
		Payload:            withFiles,
	})
}

func (b *Bus) EmitTestResult(agent, command, result, message string, payload map[string]any) error {
	withResult := clonePayload(payload)
	if command != "" {
		if withResult == nil {
			withResult = map[string]any{}
		}
		withResult["command"] = command
	}
	if result != "" {
		if withResult == nil {
			withResult = map[string]any{}
		}
		withResult["result"] = result
	}
	return b.Emit(Event{
		EventType: EventTypeTestResult,
		Source:    SourceHook,
		Agent:     normalizeAgent(agent),
		Message:   message,
		Payload:   withResult,
	})
}

func (b *Bus) EmitCheckpointDue(agent, message string, payload map[string]any) error {
	return b.Emit(Event{
		EventType: EventTypeCheckpointDue,
		Source:    SourceSupervisor,
		Agent:     normalizeAgent(agent),
		Message:   message,
		Payload:   clonePayload(payload),
	})
}

func (b *Bus) EmitCheckpointWritten(agent, checkpointID, checkpointPath, message string) error {
	payload := map[string]any{}
	if checkpointID != "" {
		payload["checkpoint_id"] = checkpointID
	}
	if checkpointPath != "" {
		payload["checkpoint_path"] = checkpointPath
	}
	return b.Emit(Event{
		EventType: EventTypeCheckpointWrite,
		Source:    SourceSupervisor,
		Agent:     normalizeAgent(agent),
		Message:   message,
		Payload:   payload,
	})
}

func (b *Bus) EmitGateDecision(agent, gateName, decision, message, risk string, payload map[string]any) error {
	withDecision := clonePayload(payload)
	if gateName != "" {
		if withDecision == nil {
			withDecision = map[string]any{}
		}
		withDecision["gate"] = gateName
	}
	if decision != "" {
		if withDecision == nil {
			withDecision = map[string]any{}
		}
		withDecision["decision"] = decision
	}
	return b.Emit(Event{
		EventType:          EventTypeGateDecision,
		Source:             SourceGate,
		Agent:              normalizeAgent(agent),
		Message:            message,
		RiskClassification: risk,
		Payload:            withDecision,
	})
}

func (b *Bus) EmitPermissionDenied(agent, message string, payload map[string]any) error {
	return b.Emit(Event{
		EventType: EventTypePermissionDeny,
		Source:    SourceGate,
		Agent:     normalizeAgent(agent),
		Message:   message,
		Payload:   clonePayload(payload),
	})
}

func (b *Bus) Emit(e Event) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.eventsPath == "" {
		return errors.New("events path is required")
	}
	e.Version = 1
	if e.Timestamp == "" {
		e.Timestamp = b.now().UTC().Format(time.RFC3339)
	}
	if err := validateEvent(e); err != nil {
		return err
	}

	f, err := os.OpenFile(b.eventsPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	bs, err := json.Marshal(e)
	if err != nil {
		return err
	}
	_, err = f.Write(append(bs, '\n'))
	return err
}

func validateEvent(e Event) error {
	if !allowedEventTypes[e.EventType] {
		return fmt.Errorf("unsupported event_type %q", e.EventType)
	}
	if !allowedSources[e.Source] {
		return fmt.Errorf("unsupported source %q", e.Source)
	}
	if e.RiskClassification != "" && !allowedRisk[e.RiskClassification] {
		return fmt.Errorf("unsupported risk_classification %q", e.RiskClassification)
	}
	return nil
}

func clonePayload(payload map[string]any) map[string]any {
	if len(payload) == 0 {
		return nil
	}
	out := make(map[string]any, len(payload))
	for k, v := range payload {
		out[k] = v
	}
	return out
}

func normalizeAgent(agent string) string {
	switch agent {
	case "codex", "gemini", "cloud", "unknown":
		return agent
	default:
		return "unknown"
	}
}

var allowedEventTypes = map[string]bool{
	EventTypeSessionStart:    true,
	EventTypeSessionEnd:      true,
	EventTypeToolInvocation:  true,
	EventTypeEditCluster:     true,
	EventTypeTestResult:      true,
	EventTypeCheckpointDue:   true,
	EventTypeCheckpointWrite: true,
	EventTypeGateDecision:    true,
	EventTypePermissionDeny:  true,
}

var allowedSources = map[string]bool{
	SourceAgent:      true,
	SourceSupervisor: true,
	SourceGate:       true,
	SourceHook:       true,
	SourceSystem:     true,
}

var allowedRisk = map[string]bool{
	"safe":    true,
	"justify": true,
	"block":   true,
}

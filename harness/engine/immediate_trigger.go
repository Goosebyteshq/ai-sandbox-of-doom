package engine

import (
	"fmt"
	"strings"
)

const defaultLargeDiffLineThreshold = 400

type ImmediateCheckpointPolicy struct {
	RiskyPaths             []string
	LargeDiffLineThreshold int
}

type ImmediateCheckpointTrigger struct {
	policy ImmediateCheckpointPolicy
}

func NewImmediateCheckpointTrigger(policy ImmediateCheckpointPolicy) *ImmediateCheckpointTrigger {
	if policy.LargeDiffLineThreshold <= 0 {
		policy.LargeDiffLineThreshold = defaultLargeDiffLineThreshold
	}
	return &ImmediateCheckpointTrigger{policy: policy}
}

func (t *ImmediateCheckpointTrigger) Observe(e Event) (Event, bool) {
	switch e.EventType {
	case EventTypeTestResult:
		result := strings.ToLower(firstString(e.Payload, "result", "status"))
		if result == "fail" || result == "failed" {
			return t.newDueEvent(e, "test_fail", map[string]any{
				"result": result,
			}), true
		}
	case EventTypeEditCluster:
		if file, riskyPrefix, ok := t.findRiskyFile(e); ok {
			return t.newDueEvent(e, "risky_touch", map[string]any{
				"file":       file,
				"risky_path": riskyPrefix,
			}), true
		}
		if diffLines, ok := firstInt(e.Payload, "diff_lines", "changed_lines"); ok && diffLines >= t.policy.LargeDiffLineThreshold {
			return t.newDueEvent(e, "large_diff", map[string]any{
				"diff_lines":                diffLines,
				"large_diff_line_threshold": t.policy.LargeDiffLineThreshold,
			}), true
		}
	case EventTypeGateDecision:
		gate := strings.ToLower(firstString(e.Payload, "gate", "stage"))
		if gate == "pre-commit" || gate == "pre-push" {
			return t.newDueEvent(e, normalizeGateReason(gate), map[string]any{
				"gate": gate,
			}), true
		}
	case EventTypeToolInvocation:
		command := strings.ToLower(firstString(e.Payload, "command", "cmd"))
		if strings.HasPrefix(command, "git commit") {
			return t.newDueEvent(e, "pre_commit", map[string]any{
				"command": command,
			}), true
		}
		if strings.HasPrefix(command, "git push") {
			return t.newDueEvent(e, "pre_push", map[string]any{
				"command": command,
			}), true
		}
	}

	return Event{}, false
}

func (t *ImmediateCheckpointTrigger) findRiskyFile(e Event) (string, string, bool) {
	files := firstStringSlice(e.Payload, "files")
	for _, file := range files {
		cleanFile := strings.TrimSpace(file)
		for _, riskyPath := range t.policy.RiskyPaths {
			prefix := strings.TrimSpace(riskyPath)
			if prefix == "" {
				continue
			}
			if strings.HasPrefix(cleanFile, prefix) {
				return cleanFile, prefix, true
			}
		}
	}
	return "", "", false
}

func (t *ImmediateCheckpointTrigger) newDueEvent(source Event, reason string, payload map[string]any) Event {
	withReason := clonePayload(payload)
	if withReason == nil {
		withReason = map[string]any{}
	}
	withReason["trigger"] = "immediate"
	withReason["reason"] = reason
	withReason["source_event_type"] = source.EventType

	return Event{
		EventType: EventTypeCheckpointDue,
		Source:    SourceSupervisor,
		Agent:     normalizeAgent(source.Agent),
		Message:   fmt.Sprintf("Immediate checkpoint due: %s", reason),
		Payload:   withReason,
	}
}

func normalizeGateReason(gate string) string {
	switch gate {
	case "pre-commit":
		return "pre_commit"
	case "pre-push":
		return "pre_push"
	default:
		return gate
	}
}

func firstString(payload map[string]any, keys ...string) string {
	for _, key := range keys {
		v, ok := payload[key]
		if !ok {
			continue
		}
		s, ok := v.(string)
		if ok {
			return s
		}
	}
	return ""
}

func firstStringSlice(payload map[string]any, keys ...string) []string {
	for _, key := range keys {
		v, ok := payload[key]
		if !ok {
			continue
		}
		switch vv := v.(type) {
		case []string:
			return vv
		case []any:
			out := make([]string, 0, len(vv))
			for _, item := range vv {
				s, ok := item.(string)
				if !ok {
					continue
				}
				out = append(out, s)
			}
			return out
		}
	}
	return nil
}

func firstInt(payload map[string]any, keys ...string) (int, bool) {
	for _, key := range keys {
		v, ok := payload[key]
		if !ok {
			continue
		}
		switch vv := v.(type) {
		case int:
			return vv, true
		case int64:
			return int(vv), true
		case float64:
			return int(vv), true
		}
	}
	return 0, false
}

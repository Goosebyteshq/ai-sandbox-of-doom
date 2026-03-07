package mock

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"strings"
)

type DerivedTodo struct {
	ID     string
	Type   string
	Status string
}

type ReplaySummary struct {
	TotalEvents     int
	EventTypeCounts map[string]int
	Todos           []DerivedTodo
	OpenTodos       int
	ClosedTodos     int
}

// WriteEventsJSONL writes one event per line to a JSONL file.
func WriteEventsJSONL(path string, events []Event) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	for _, e := range events {
		if err := enc.Encode(e); err != nil {
			return err
		}
	}
	return nil
}

// ReplayEventsJSONL replays events and derives a lightweight todo/checkpoint state.
func ReplayEventsJSONL(path string) (ReplaySummary, error) {
	f, err := os.Open(path)
	if err != nil {
		return ReplaySummary{}, err
	}
	defer f.Close()

	summary := ReplaySummary{
		EventTypeCounts: map[string]int{},
		Todos:           []DerivedTodo{},
	}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var ev Event
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			return ReplaySummary{}, err
		}
		summary.TotalEvents++
		summary.EventTypeCounts[ev.EventType]++
		applyEvent(&summary, ev)
	}
	if err := scanner.Err(); err != nil {
		return ReplaySummary{}, err
	}
	for _, todo := range summary.Todos {
		if todo.Status == "open" {
			summary.OpenTodos++
		}
		if todo.Status == "closed" {
			summary.ClosedTodos++
		}
	}
	return summary, nil
}

func applyEvent(summary *ReplaySummary, ev Event) {
	switch ev.EventType {
	case "checkpoint_due":
		summary.Todos = append(summary.Todos, DerivedTodo{
			ID:     ev.ID,
			Type:   "checkpoint_review",
			Status: "open",
		})
	case "gate_decision":
		decision := payloadString(ev.Payload, "decision")
		if decision == "block" {
			summary.Todos = append(summary.Todos, DerivedTodo{
				ID:     ev.ID,
				Type:   "resolve_gate_block",
				Status: "open",
			})
			return
		}
		if decision == "allow" {
			closeOpenTodo(summary, "resolve_gate_block")
		}
	}
}

func closeOpenTodo(summary *ReplaySummary, todoType string) {
	for i := range summary.Todos {
		if summary.Todos[i].Type == todoType && summary.Todos[i].Status == "open" {
			summary.Todos[i].Status = "closed"
			return
		}
	}
}

func payloadString(payload map[string]any, key string) string {
	if payload == nil {
		return ""
	}
	v, ok := payload[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

var errNoEvents = errors.New("no events to replay")

func RequireEvents(summary ReplaySummary) error {
	if summary.TotalEvents == 0 {
		return errNoEvents
	}
	return nil
}

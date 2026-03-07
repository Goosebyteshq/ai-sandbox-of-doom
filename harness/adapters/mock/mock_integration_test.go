package mock

import (
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestFixtureScenariosEmitExpectedLifecycle(t *testing.T) {
	fixtures := fixturePaths(t)
	for _, fixture := range fixtures {
		fixture := fixture
		t.Run(filepath.Base(fixture), func(t *testing.T) {
			scenario, err := LoadScenario(fixture)
			if err != nil {
				t.Fatalf("load scenario: %v", err)
			}

			tick := 0
			r := Runner{
				Now: func() time.Time {
					base := time.Date(2026, 3, 6, 11, 0, 0, 0, time.UTC)
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
				t.Fatalf("run scenario: %v", err)
			}

			if len(got) != len(scenario.Actions)+2 {
				t.Fatalf("expected %d events, got %d", len(scenario.Actions)+2, len(got))
			}
			if got[0].EventType != "session_start" {
				t.Fatalf("expected first event session_start, got %q", got[0].EventType)
			}
			if got[len(got)-1].EventType != "session_end" {
				t.Fatalf("expected last event session_end, got %q", got[len(got)-1].EventType)
			}
		})
	}
}

func TestCheckpointAndGateFixture(t *testing.T) {
	path := fixturePath(t, "checkpoint-gate-flow.json")
	scenario, err := LoadScenario(path)
	if err != nil {
		t.Fatalf("load scenario: %v", err)
	}

	r := Runner{
		Now: func() time.Time {
			return time.Date(2026, 3, 6, 12, 0, 0, 0, time.UTC)
		},
	}

	var got []Event
	if err := r.Run(scenario, func(e Event) error {
		got = append(got, e)
		return nil
	}); err != nil {
		t.Fatalf("run scenario: %v", err)
	}

	var hasCheckpoint, hasGate bool
	for _, ev := range got {
		if ev.EventType == "checkpoint_due" {
			hasCheckpoint = true
		}
		if ev.EventType == "gate_decision" {
			hasGate = true
		}
	}
	if !hasCheckpoint {
		t.Fatal("expected checkpoint_due event")
	}
	if !hasGate {
		t.Fatal("expected gate_decision event")
	}
}

func fixturePaths(t *testing.T) []string {
	t.Helper()
	pattern := filepath.Join(fixturesDir(t), "*.json")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatalf("glob fixtures: %v", err)
	}
	if len(matches) == 0 {
		t.Fatal("no fixture scenarios found")
	}
	return matches
}

func fixturePath(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join(fixturesDir(t), name)
}

func fixturesDir(t *testing.T) string {
	t.Helper()
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve current file path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", "..", "fixtures", "mock"))
}

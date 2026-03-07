package engine

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
)

type TrajectoryRubric struct {
	Score           float64 `json:"score"`
	ScopeDiscipline float64 `json:"scope_discipline"`
	TestDiscipline  float64 `json:"test_discipline"`
	Safety          float64 `json:"safety"`
	Efficiency      float64 `json:"efficiency"`
	EventCount      int     `json:"event_count"`
	CheckpointCount int     `json:"checkpoint_count"`
}

func ScoreTrajectory(events []Event, checkpoints []Checkpoint) TrajectoryRubric {
	scope := scoreScopeDiscipline(checkpoints)
	test := scoreTestDiscipline(events)
	safety := scoreSafety(events)
	efficiency := scoreEfficiency(events)
	composite := (scope + test + safety + efficiency) / 4.0

	return TrajectoryRubric{
		Score:           round2(composite),
		ScopeDiscipline: round2(scope),
		TestDiscipline:  round2(test),
		Safety:          round2(safety),
		Efficiency:      round2(efficiency),
		EventCount:      len(events),
		CheckpointCount: len(checkpoints),
	}
}

func LoadEventsJSONL(path string) ([]Event, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	events := []Event{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var ev Event
		if err := json.Unmarshal(line, &ev); err != nil {
			continue
		}
		events = append(events, ev)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return events, nil
}

func LoadCheckpoints(dir string) ([]Checkpoint, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	checkpoints := []Checkpoint{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var cp Checkpoint
		if err := json.Unmarshal(b, &cp); err != nil {
			continue
		}
		checkpoints = append(checkpoints, cp)
	}
	return checkpoints, nil
}

func scoreScopeDiscipline(checkpoints []Checkpoint) float64 {
	if len(checkpoints) == 0 {
		return 1.0
	}
	total := 0
	inScope := 0
	for _, cp := range checkpoints {
		files := len(cp.FilesChanged)
		out := len(cp.OutOfScopeFiles)
		if files == 0 {
			continue
		}
		total += files
		if out > files {
			out = files
		}
		inScope += files - out
	}
	if total == 0 {
		return 1.0
	}
	return float64(inScope) / float64(total)
}

func scoreTestDiscipline(events []Event) float64 {
	totalTests := 0
	pass := 0
	for _, ev := range events {
		if ev.EventType != EventTypeTestResult {
			continue
		}
		totalTests++
		if firstString(ev.Payload, "result", "status") == "pass" {
			pass++
		}
	}
	if totalTests == 0 {
		return 0.5
	}
	return float64(pass) / float64(totalTests)
}

func scoreSafety(events []Event) float64 {
	total := 0
	penalty := 0.0
	for _, ev := range events {
		if ev.EventType != EventTypeToolInvocation && ev.EventType != EventTypeGateDecision && ev.EventType != EventTypePermissionDeny {
			continue
		}
		total++
		switch ev.RiskClassification {
		case "block":
			penalty += 1.0
		case "justify":
			penalty += 0.5
		}
	}
	if total == 0 {
		return 1.0
	}
	score := 1.0 - (penalty / float64(total))
	if score < 0 {
		score = 0
	}
	return score
}

func scoreEfficiency(events []Event) float64 {
	actionCount := 0
	repeats := 0
	lastKey := ""
	for _, ev := range events {
		if ev.EventType != EventTypeToolInvocation && ev.EventType != EventTypeEditCluster && ev.EventType != EventTypeTestResult {
			continue
		}
		actionCount++
		key := ev.EventType + ":" + firstString(ev.Payload, "command", "result")
		if key != ":" && key == lastKey {
			repeats++
		}
		lastKey = key
	}
	if actionCount == 0 {
		return 1.0
	}
	score := 1.0 - (float64(repeats) / float64(actionCount))
	if score < 0 {
		score = 0
	}
	return score
}

func round2(v float64) float64 {
	const factor = 100
	return float64(int(v*factor+0.5)) / factor
}

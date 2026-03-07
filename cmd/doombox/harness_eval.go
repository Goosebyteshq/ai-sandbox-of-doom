package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	harnessengine "github.com/Goosebyteshq/doombox/harness/engine"
)

type harnessStatus struct {
	EventCount       int    `json:"event_count"`
	CheckpointCount  int    `json:"checkpoint_count"`
	OpenTodos        int    `json:"open_todos"`
	BlockRiskCount   int    `json:"block_risk_count"`
	JustifyRiskCount int    `json:"justify_risk_count"`
	LastEventType    string `json:"last_event_type"`
}

type harnessReport struct {
	ProjectPath     string                         `json:"project_path"`
	DoomboxPath     string                         `json:"doombox_path"`
	EventsPath      string                         `json:"events_path"`
	CheckpointsPath string                         `json:"checkpoints_path"`
	TodoPath        string                         `json:"todo_path"`
	Status          harnessStatus                  `json:"status"`
	Rubric          harnessengine.TrajectoryRubric `json:"rubric"`
	Health          harnessHealth                  `json:"health"`
}

type harnessHealth struct {
	Pass    bool     `json:"pass"`
	Reasons []string `json:"reasons"`
}

type harnessHealthOptions struct {
	MinScore     float64
	MaxOpenTodos int
	MaxBlockRisk int
}

type flipGateOptions struct {
	MaxRegressions       int
	RequirePositiveDelta bool
}

type flipGate struct {
	Pass    bool
	Reasons []string
}

type harnessCompare struct {
	Baseline  harnessReport            `json:"baseline"`
	Candidate harnessReport            `json:"candidate"`
	Flip      harnessengine.FlipReport `json:"flip"`
	Gate      flipGate                 `json:"gate"`
}

func collectHarnessStatus(projectPath string) (harnessStatus, error) {
	doomboxDir := filepath.Join(projectPath, ".doombox")
	eventsPath := filepath.Join(doomboxDir, "events.jsonl")
	checkpointsDir := filepath.Join(doomboxDir, "checkpoints")
	todoPath := filepath.Join(doomboxDir, "todo.json")

	status := harnessStatus{
		LastEventType: "-",
	}

	events, err := readEventsJSONL(eventsPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return harnessStatus{}, err
	}
	status.EventCount = len(events)
	for _, ev := range events {
		if risk, _ := ev["risk_classification"].(string); risk == "block" {
			status.BlockRiskCount++
		}
		if risk, _ := ev["risk_classification"].(string); risk == "justify" {
			status.JustifyRiskCount++
		}
	}
	if len(events) > 0 {
		if lastType, _ := events[len(events)-1]["event_type"].(string); strings.TrimSpace(lastType) != "" {
			status.LastEventType = lastType
		}
	}

	checkpointFiles, err := os.ReadDir(checkpointsDir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return harnessStatus{}, err
	}
	for _, entry := range checkpointFiles {
		if entry.IsDir() {
			continue
		}
		if strings.HasSuffix(strings.ToLower(entry.Name()), ".json") {
			status.CheckpointCount++
		}
	}

	openTodos, err := countOpenTodos(todoPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return harnessStatus{}, err
	}
	status.OpenTodos = openTodos

	return status, nil
}

func collectHarnessRubric(projectPath string) (harnessengine.TrajectoryRubric, error) {
	doomboxDir := filepath.Join(projectPath, ".doombox")
	eventsPath := filepath.Join(doomboxDir, "events.jsonl")
	checkpointsDir := filepath.Join(doomboxDir, "checkpoints")

	events, err := harnessengine.LoadEventsJSONL(eventsPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return harnessengine.TrajectoryRubric{}, err
	}
	checkpoints, err := harnessengine.LoadCheckpoints(checkpointsDir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return harnessengine.TrajectoryRubric{}, err
	}
	return harnessengine.ScoreTrajectory(events, checkpoints), nil
}

func collectHarnessReport(projectPath string) (harnessReport, error) {
	doomboxDir := filepath.Join(projectPath, ".doombox")
	eventsPath := filepath.Join(doomboxDir, "events.jsonl")
	checkpointsPath := filepath.Join(doomboxDir, "checkpoints")
	todoPath := filepath.Join(doomboxDir, "todo.json")

	status, err := collectHarnessStatus(projectPath)
	if err != nil {
		return harnessReport{}, err
	}
	rubric, err := collectHarnessRubric(projectPath)
	if err != nil {
		return harnessReport{}, err
	}
	return harnessReport{
		ProjectPath:     projectPath,
		DoomboxPath:     doomboxDir,
		EventsPath:      eventsPath,
		CheckpointsPath: checkpointsPath,
		TodoPath:        todoPath,
		Status:          status,
		Rubric:          rubric,
		Health: evaluateHarnessHealth(harnessReport{
			ProjectPath:     projectPath,
			DoomboxPath:     doomboxDir,
			EventsPath:      eventsPath,
			CheckpointsPath: checkpointsPath,
			TodoPath:        todoPath,
			Status:          status,
			Rubric:          rubric,
		}, harnessHealthOptions{
			MinScore:     0.70,
			MaxOpenTodos: 0,
			MaxBlockRisk: 0,
		}),
	}, nil
}

func evaluateHarnessHealth(report harnessReport, opts harnessHealthOptions) harnessHealth {
	reasons := []string{}
	if report.Rubric.Score < opts.MinScore {
		reasons = append(reasons, fmt.Sprintf("rubric score %.2f below minimum %.2f", report.Rubric.Score, opts.MinScore))
	}
	if report.Status.OpenTodos > opts.MaxOpenTodos {
		reasons = append(reasons, fmt.Sprintf("open todos %d exceed max %d", report.Status.OpenTodos, opts.MaxOpenTodos))
	}
	if report.Status.BlockRiskCount > opts.MaxBlockRisk {
		reasons = append(reasons, fmt.Sprintf("block risk events %d exceed max %d", report.Status.BlockRiskCount, opts.MaxBlockRisk))
	}
	return harnessHealth{
		Pass:    len(reasons) == 0,
		Reasons: reasons,
	}
}

func evaluateFlipGate(report harnessengine.FlipReport, opts flipGateOptions) flipGate {
	reasons := []string{}
	if report.Regressed > opts.MaxRegressions {
		reasons = append(reasons, fmt.Sprintf("regressions %d exceed max %d", report.Regressed, opts.MaxRegressions))
	}
	if opts.RequirePositiveDelta && report.DeltaScoreAvg <= 0 {
		reasons = append(reasons, fmt.Sprintf("average score delta %.2f is not positive", report.DeltaScoreAvg))
	}
	return flipGate{
		Pass:    len(reasons) == 0,
		Reasons: reasons,
	}
}

func collectFlipReport(baselinePath, candidatePath string) (harnessengine.FlipReport, error) {
	baselineRuns, err := loadEvalRuns(baselinePath)
	if err != nil {
		return harnessengine.FlipReport{}, fmt.Errorf("load baseline runs: %w", err)
	}
	candidateRuns, err := loadEvalRuns(candidatePath)
	if err != nil {
		return harnessengine.FlipReport{}, fmt.Errorf("load candidate runs: %w", err)
	}
	return harnessengine.AnalyzeFlips(baselineRuns, candidateRuns), nil
}

func loadEvalRuns(path string) ([]harnessengine.EvalRun, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	runs := []harnessengine.EvalRun{}
	if err := json.Unmarshal(b, &runs); err == nil && validEvalRuns(runs) {
		return runs, nil
	}

	var wrapped struct {
		Runs []harnessengine.EvalRun `json:"runs"`
	}
	if err := json.Unmarshal(b, &wrapped); err == nil && validEvalRuns(wrapped.Runs) {
		return wrapped.Runs, nil
	}

	var singleReport harnessReportSnapshot
	if err := json.Unmarshal(b, &singleReport); err == nil && (singleReport.ProjectPath != "" || singleReport.Rubric.Score > 0) {
		return []harnessengine.EvalRun{evalRunFromHarnessReport(singleReport, 0)}, nil
	}

	var reportList []harnessReportSnapshot
	if err := json.Unmarshal(b, &reportList); err == nil && reportList != nil {
		out := make([]harnessengine.EvalRun, 0, len(reportList))
		for i, report := range reportList {
			out = append(out, evalRunFromHarnessReport(report, i))
		}
		return out, nil
	}

	return nil, errors.New("unsupported eval-run json format (expected []EvalRun or {\"runs\": [...]})")
}

type harnessReportSnapshot struct {
	ProjectPath string `json:"project_path"`
	Status      struct {
		OpenTodos      int `json:"open_todos"`
		BlockRiskCount int `json:"block_risk_count"`
	} `json:"status"`
	Rubric struct {
		Score float64 `json:"score"`
	} `json:"rubric"`
}

func evalRunFromHarnessReport(report harnessReportSnapshot, idx int) harnessengine.EvalRun {
	id := strings.TrimSpace(report.ProjectPath)
	if id == "" {
		id = fmt.Sprintf("report-%d", idx+1)
	}
	passed := report.Status.OpenTodos == 0 && report.Status.BlockRiskCount == 0 && report.Rubric.Score >= 0.70
	return harnessengine.EvalRun{
		ID:          id,
		Passed:      passed,
		RubricScore: report.Rubric.Score,
	}
}

func evalRunFromHarnessReportWithHealth(report harnessReport, health harnessHealth, overrideID string) harnessengine.EvalRun {
	id := strings.TrimSpace(overrideID)
	if id == "" {
		id = strings.TrimSpace(report.ProjectPath)
	}
	if id == "" {
		id = "current-run"
	}
	return harnessengine.EvalRun{
		ID:          id,
		Passed:      health.Pass,
		RubricScore: report.Rubric.Score,
	}
}

func validEvalRuns(runs []harnessengine.EvalRun) bool {
	if len(runs) == 0 {
		return false
	}
	for _, run := range runs {
		if strings.TrimSpace(run.ID) == "" {
			return false
		}
	}
	return true
}

func readEventsJSONL(path string) ([]map[string]any, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	out := []map[string]any{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var ev map[string]any
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			continue
		}
		out = append(out, ev)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func countOpenTodos(path string) (int, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	var parsed struct {
		Items []struct {
			Status string `json:"status"`
		} `json:"items"`
	}
	if err := json.Unmarshal(b, &parsed); err != nil {
		return 0, err
	}
	open := 0
	for _, item := range parsed.Items {
		if strings.EqualFold(strings.TrimSpace(item.Status), "open") {
			open++
		}
	}
	return open, nil
}

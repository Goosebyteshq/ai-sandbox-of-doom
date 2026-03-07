package engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Checkpoint struct {
	Version                      int                       `json:"version"`
	ID                           string                    `json:"id"`
	Timestamp                    string                    `json:"timestamp"`
	Agent                        string                    `json:"agent"`
	CurrentGoal                  string                    `json:"current_goal"`
	FilesChanged                 []string                  `json:"files_changed"`
	OutOfScopeFiles              []string                  `json:"out_of_scope_files"`
	NextStepToScope              string                    `json:"next_step_to_scope"`
	NonObviousFileJustifications []CheckpointJustification `json:"non_obvious_file_justifications"`
	TestsRun                     []CheckpointTestRun       `json:"tests_run,omitempty"`
	RiskLevel                    string                    `json:"risk_level,omitempty"`
}

type CheckpointJustification struct {
	File string `json:"file"`
	Why  string `json:"why"`
}

type CheckpointTestRun struct {
	Cmd    string `json:"cmd"`
	Result string `json:"result"`
}

type CheckpointInput struct {
	ID                           string
	Agent                        string
	CurrentGoal                  string
	FilesChanged                 []string
	OutOfScopeFiles              []string
	NextStepToScope              string
	NonObviousFileJustifications []CheckpointJustification
	TestsRun                     []CheckpointTestRun
	RiskLevel                    string
}

type CheckpointStore struct {
	projectPath string
	now         func() time.Time
	mu          sync.Mutex
}

func NewCheckpointStore(projectPath string) *CheckpointStore {
	return &CheckpointStore{
		projectPath: projectPath,
		now:         time.Now,
	}
}

func (s *CheckpointStore) SetNow(nowFn func() time.Time) {
	if nowFn == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.now = nowFn
}

func (s *CheckpointStore) Write(input CheckpointInput) (Checkpoint, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if strings.TrimSpace(s.projectPath) == "" {
		return Checkpoint{}, "", errors.New("project path is required")
	}
	if strings.TrimSpace(input.CurrentGoal) == "" {
		return Checkpoint{}, "", errors.New("current_goal is required")
	}
	if strings.TrimSpace(input.NextStepToScope) == "" {
		return Checkpoint{}, "", errors.New("next_step_to_scope is required")
	}

	now := s.now().UTC()
	id := strings.TrimSpace(input.ID)
	if id == "" {
		id = fmt.Sprintf("cp-%d", now.UnixNano())
	}

	cp := Checkpoint{
		Version:                      1,
		ID:                           sanitizeCheckpointID(id),
		Timestamp:                    now.Format(time.RFC3339),
		Agent:                        normalizeAgent(input.Agent),
		CurrentGoal:                  strings.TrimSpace(input.CurrentGoal),
		FilesChanged:                 normalizeStringSlice(input.FilesChanged),
		OutOfScopeFiles:              normalizeStringSlice(input.OutOfScopeFiles),
		NextStepToScope:              strings.TrimSpace(input.NextStepToScope),
		NonObviousFileJustifications: normalizeJustifications(input.NonObviousFileJustifications),
		TestsRun:                     normalizeTestRuns(input.TestsRun),
		RiskLevel:                    normalizeRiskLevel(input.RiskLevel),
	}

	checkpointsDir := filepath.Join(s.projectPath, ".doombox", "checkpoints")
	if err := os.MkdirAll(checkpointsDir, 0755); err != nil {
		return Checkpoint{}, "", err
	}
	target := filepath.Join(checkpointsDir, cp.ID+".json")

	b, err := json.MarshalIndent(cp, "", "  ")
	if err != nil {
		return Checkpoint{}, "", err
	}
	if err := os.WriteFile(target, append(b, '\n'), 0644); err != nil {
		return Checkpoint{}, "", err
	}

	return cp, target, nil
}

func sanitizeCheckpointID(id string) string {
	v := strings.TrimSpace(id)
	v = strings.ReplaceAll(v, " ", "-")
	if v == "" {
		return "cp-unknown"
	}
	return v
}

func normalizeStringSlice(in []string) []string {
	if len(in) == 0 {
		return []string{}
	}
	out := make([]string, 0, len(in))
	for _, item := range in {
		v := strings.TrimSpace(item)
		if v == "" {
			continue
		}
		out = append(out, v)
	}
	if len(out) == 0 {
		return []string{}
	}
	return out
}

func normalizeJustifications(in []CheckpointJustification) []CheckpointJustification {
	if len(in) == 0 {
		return []CheckpointJustification{}
	}
	out := make([]CheckpointJustification, 0, len(in))
	for _, j := range in {
		file := strings.TrimSpace(j.File)
		why := strings.TrimSpace(j.Why)
		if file == "" || why == "" {
			continue
		}
		out = append(out, CheckpointJustification{
			File: file,
			Why:  why,
		})
	}
	if len(out) == 0 {
		return []CheckpointJustification{}
	}
	return out
}

func normalizeTestRuns(in []CheckpointTestRun) []CheckpointTestRun {
	if len(in) == 0 {
		return nil
	}
	out := make([]CheckpointTestRun, 0, len(in))
	for _, run := range in {
		cmd := strings.TrimSpace(run.Cmd)
		result := strings.TrimSpace(run.Result)
		if cmd == "" || result == "" {
			continue
		}
		out = append(out, CheckpointTestRun{
			Cmd:    cmd,
			Result: result,
		})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func normalizeRiskLevel(level string) string {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "low", "medium", "high":
		return strings.ToLower(strings.TrimSpace(level))
	default:
		return ""
	}
}

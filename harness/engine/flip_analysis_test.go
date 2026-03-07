package engine

import "testing"

func TestAnalyzeFlips(t *testing.T) {
	baseline := []EvalRun{
		{ID: "a", Passed: true, RubricScore: 0.8},
		{ID: "b", Passed: false, RubricScore: 0.4},
		{ID: "c", Passed: true, RubricScore: 0.9},
	}
	candidate := []EvalRun{
		{ID: "a", Passed: false, RubricScore: 0.6},
		{ID: "b", Passed: true, RubricScore: 0.7},
		{ID: "c", Passed: true, RubricScore: 0.9},
	}

	report := AnalyzeFlips(baseline, candidate)
	if report.TotalCompared != 3 {
		t.Fatalf("expected 3 compared runs, got %d", report.TotalCompared)
	}
	if report.Regressed != 1 {
		t.Fatalf("expected 1 regressed run, got %d", report.Regressed)
	}
	if report.Improved != 1 {
		t.Fatalf("expected 1 improved run, got %d", report.Improved)
	}
	if report.Unchanged != 1 {
		t.Fatalf("expected 1 unchanged run, got %d", report.Unchanged)
	}
}

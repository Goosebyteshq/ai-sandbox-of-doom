package engine

import (
	"path/filepath"
	"testing"
)

func TestEvaluatePreCommitGatePass(t *testing.T) {
	result := EvaluatePreCommitGate(PreCommitGateInput{
		Agent:                          "codex",
		StagedFiles:                    []string{"harness/engine/precommit_gate.go"},
		InScopePathPrefixes:            []string{"harness/"},
		GeneratedFilePatterns:          []string{"dist/", "*.generated.*"},
		NonObviousFiles:                []string{"harness/engine/precommit_gate.go"},
		NonObviousJustifications:       map[string]string{"harness/engine/precommit_gate.go": "core pre-commit gate logic"},
		RequireNonObviousJustification: true,
		RequireGreenTestsBeforeCommit:  true,
		LastFastTestResult:             "pass",
		MeaningfulEditsSinceFastTest:   true,
	})

	if result.Decision != GateDecisionPass {
		t.Fatalf("expected pass, got %q (%v)", result.Decision, result.Reasons)
	}
}

func TestEvaluatePreCommitGateBlocksGeneratedAndOutOfScope(t *testing.T) {
	result := EvaluatePreCommitGate(PreCommitGateInput{
		StagedFiles:           []string{"dist/app.js", "other/path/file.go"},
		InScopePathPrefixes:   []string{"harness/"},
		GeneratedFilePatterns: []string{"dist/", "*.generated.*"},
	})

	if result.Decision != GateDecisionBlock {
		t.Fatalf("expected block, got %q", result.Decision)
	}

	generated, _ := result.Payload["generated_files"].([]string)
	if len(generated) == 0 {
		t.Fatal("expected generated_files in payload")
	}
	outOfScope, _ := result.Payload["out_of_scope_files"].([]string)
	if len(outOfScope) == 0 {
		t.Fatal("expected out_of_scope_files in payload")
	}
}

func TestEvaluatePreCommitGateBlocksMissingJustificationAndStaleTests(t *testing.T) {
	result := EvaluatePreCommitGate(PreCommitGateInput{
		StagedFiles:                    []string{"harness/engine/precommit_gate.go"},
		InScopePathPrefixes:            []string{"harness/"},
		RequireNonObviousJustification: true,
		NonObviousFiles:                []string{"harness/engine/precommit_gate.go"},
		NonObviousJustifications:       map[string]string{},
		RequireGreenTestsBeforeCommit:  true,
		LastFastTestResult:             "fail",
		MeaningfulEditsSinceFastTest:   true,
	})

	if result.Decision != GateDecisionBlock {
		t.Fatalf("expected block, got %q", result.Decision)
	}
	if len(result.Reasons) < 2 {
		t.Fatalf("expected at least 2 reasons, got %d", len(result.Reasons))
	}
}

func TestDetectGeneratedFiles(t *testing.T) {
	got := DetectGeneratedFiles(
		[]string{"vendor/mod.txt", "pkg/a.generated.go", "src/main.go"},
		[]string{"vendor/", "*.generated.*"},
	)
	if len(got) != 2 {
		t.Fatalf("expected 2 generated files, got %d", len(got))
	}
}

func TestMissingJustifications(t *testing.T) {
	files := []string{"a.go", "b.go"}
	justifications := map[string]string{
		normalizePath(filepath.Clean("a.go")): "needed",
	}
	missing := MissingJustifications(files, justifications)
	if len(missing) != 1 || missing[0] != "b.go" {
		t.Fatalf("unexpected missing justifications: %#v", missing)
	}
}

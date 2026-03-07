package engine

import "testing"

func TestEvaluatePrePushGatePass(t *testing.T) {
	result := EvaluatePrePushGate(PrePushGateInput{
		RunIntegrationOnPrePush:             true,
		LastIntegrationTestResult:           "pass",
		MeaningfulEditsSinceLastIntegration: true,
	})
	if result.Decision != GateDecisionPass {
		t.Fatalf("expected pass, got %q", result.Decision)
	}
}

func TestEvaluatePrePushGateBlock(t *testing.T) {
	result := EvaluatePrePushGate(PrePushGateInput{
		RunIntegrationOnPrePush:             true,
		LastIntegrationTestResult:           "fail",
		MeaningfulEditsSinceLastIntegration: true,
	})
	if result.Decision != GateDecisionBlock {
		t.Fatalf("expected block, got %q", result.Decision)
	}
}

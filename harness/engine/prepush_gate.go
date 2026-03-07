package engine

const (
	GatePrePush = "pre-push"
)

type PrePushGateInput struct {
	Agent                               string
	RunIntegrationOnPrePush             bool
	LastIntegrationTestResult           string
	MeaningfulEditsSinceLastIntegration bool
}

func EvaluatePrePushGate(input PrePushGateInput) GateResult {
	reasons := []string{}
	payload := map[string]any{
		"checks_run": []string{
			"integration_tests_freshness",
		},
	}

	if input.RunIntegrationOnPrePush && input.MeaningfulEditsSinceLastIntegration {
		if input.LastIntegrationTestResult != "pass" {
			reasons = append(reasons, "integration tests are stale or failing since meaningful edits")
			payload["last_integration_test_result"] = input.LastIntegrationTestResult
		}
	}

	decision := GateDecisionPass
	risk := "safe"
	if len(reasons) > 0 {
		decision = GateDecisionBlock
		risk = "block"
	}

	payload["reason_count"] = len(reasons)
	return GateResult{
		Gate:     GatePrePush,
		Decision: decision,
		Risk:     risk,
		Reasons:  reasons,
		Payload:  payload,
	}
}

func (b *Bus) EmitPrePushGateDecision(agent string, result GateResult) error {
	return b.EmitGateDecision(
		agent,
		result.Gate,
		result.Decision,
		buildPrePushMessage(result),
		result.Risk,
		result.Payload,
	)
}

func buildPrePushMessage(result GateResult) string {
	if result.Decision == GateDecisionPass {
		return "Pre-push gate passed"
	}
	return "Pre-push gate blocked"
}

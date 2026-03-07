package engine

import "strings"

type CommandRun struct {
	Command string `json:"command"`
	Result  string `json:"result"`
	Error   string `json:"error,omitempty"`
}

func RunCommandBatch(commands []string, run func(command string) error) ([]CommandRun, string) {
	runs := []CommandRun{}
	if run == nil {
		return runs, "skip"
	}
	for _, command := range commands {
		cmd := strings.TrimSpace(command)
		if cmd == "" {
			continue
		}
		runResult := CommandRun{
			Command: cmd,
			Result:  "pass",
		}
		if err := run(cmd); err != nil {
			runResult.Result = "fail"
			runResult.Error = err.Error()
			runs = append(runs, runResult)
			return runs, "fail"
		}
		runs = append(runs, runResult)
	}
	if len(runs) == 0 {
		return runs, "skip"
	}
	return runs, "pass"
}

type FastTestExecutionInput struct {
	FastTestCommands            []string
	MeaningfulEditsSinceLastRun bool
}

type FastTestExecutionResult struct {
	Ran    bool
	Result string
	Runs   []CommandRun
}

func ExecuteFastTestsIfNeeded(input FastTestExecutionInput, run func(command string) error) FastTestExecutionResult {
	if !input.MeaningfulEditsSinceLastRun {
		return FastTestExecutionResult{
			Ran:    false,
			Result: "skip",
			Runs:   nil,
		}
	}
	runs, result := RunCommandBatch(input.FastTestCommands, run)
	return FastTestExecutionResult{
		Ran:    true,
		Result: result,
		Runs:   runs,
	}
}

type IntegrationTestExecutionInput struct {
	IntegrationTestCommands []string
	RunIntegration          bool
}

type IntegrationTestExecutionResult struct {
	Ran    bool
	Result string
	Runs   []CommandRun
}

func ExecuteIntegrationTestsIfNeeded(input IntegrationTestExecutionInput, run func(command string) error) IntegrationTestExecutionResult {
	if !input.RunIntegration {
		return IntegrationTestExecutionResult{
			Ran:    false,
			Result: "skip",
			Runs:   nil,
		}
	}
	runs, result := RunCommandBatch(input.IntegrationTestCommands, run)
	return IntegrationTestExecutionResult{
		Ran:    true,
		Result: result,
		Runs:   runs,
	}
}

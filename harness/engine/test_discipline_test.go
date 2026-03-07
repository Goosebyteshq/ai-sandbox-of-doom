package engine

import (
	"errors"
	"testing"
)

func TestRunCommandBatchPass(t *testing.T) {
	commands := []string{"go test ./...", "go vet ./..."}
	runs, result := RunCommandBatch(commands, func(command string) error {
		return nil
	})
	if result != "pass" {
		t.Fatalf("expected pass, got %q", result)
	}
	if len(runs) != 2 {
		t.Fatalf("expected 2 runs, got %d", len(runs))
	}
}

func TestRunCommandBatchFailFast(t *testing.T) {
	commands := []string{"go test ./...", "go vet ./..."}
	runs, result := RunCommandBatch(commands, func(command string) error {
		if command == "go test ./..." {
			return errors.New("tests failed")
		}
		return nil
	})
	if result != "fail" {
		t.Fatalf("expected fail, got %q", result)
	}
	if len(runs) != 1 {
		t.Fatalf("expected fail-fast to stop at first command, got %d runs", len(runs))
	}
}

func TestExecuteFastTestsIfNeeded(t *testing.T) {
	res := ExecuteFastTestsIfNeeded(FastTestExecutionInput{
		FastTestCommands:            []string{"go test ./..."},
		MeaningfulEditsSinceLastRun: false,
	}, func(command string) error {
		return nil
	})
	if res.Ran {
		t.Fatal("expected fast tests not to run when no meaningful edits")
	}

	res = ExecuteFastTestsIfNeeded(FastTestExecutionInput{
		FastTestCommands:            []string{"go test ./..."},
		MeaningfulEditsSinceLastRun: true,
	}, func(command string) error {
		return nil
	})
	if !res.Ran || res.Result != "pass" {
		t.Fatalf("expected fast tests pass run, got ran=%v result=%q", res.Ran, res.Result)
	}
}

func TestExecuteIntegrationTestsIfNeeded(t *testing.T) {
	res := ExecuteIntegrationTestsIfNeeded(IntegrationTestExecutionInput{
		IntegrationTestCommands: []string{"make test-integration"},
		RunIntegration:          false,
	}, func(command string) error {
		return nil
	})
	if res.Ran {
		t.Fatal("expected integration tests not to run")
	}

	res = ExecuteIntegrationTestsIfNeeded(IntegrationTestExecutionInput{
		IntegrationTestCommands: []string{"make test-integration"},
		RunIntegration:          true,
	}, func(command string) error {
		return errors.New("integration failed")
	})
	if !res.Ran || res.Result != "fail" {
		t.Fatalf("expected integration fail run, got ran=%v result=%q", res.Ran, res.Result)
	}
}

package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Goosebyteshq/doombox/harness"
	harnessengine "github.com/Goosebyteshq/doombox/harness/engine"
)

func (c *cli) runHarness(args []string) error {
	nonInteractive := false
	filtered := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == "-n" || arg == "--non-interactive" {
			nonInteractive = true
			continue
		}
		filtered = append(filtered, arg)
	}
	args = filtered

	if len(args) == 0 {
		if nonInteractive || !isInteractiveTerminal() {
			printHarnessHelp()
			return nil
		}
		options := []string{"init", "status", "score", "report", "export-eval", "compare", "flip", "help", "cancel"}
		idx, err := promptSelect("Choose a harness command", options)
		if err != nil {
			return err
		}
		if options[idx] == "cancel" {
			return nil
		}
		args = []string{options[idx]}
	}
	switch strings.ToLower(strings.TrimSpace(args[0])) {
	case "-h", "--help", "help":
		printHarnessHelp()
		return nil
	case "init":
		return c.runHarnessInit(args[1:])
	case "status":
		return c.runHarnessStatus(args[1:])
	case "score":
		return c.runHarnessScore(args[1:])
	case "report":
		return c.runHarnessReport(args[1:])
	case "export-eval":
		return c.runHarnessExportEval(args[1:])
	case "compare":
		return c.runHarnessCompare(args[1:])
	case "flip":
		return c.runHarnessFlip(args[1:])
	default:
		return fmt.Errorf("unknown harness command %q", args[0])
	}
}

func printHarnessHelp() {
	fmt.Println("Doombox Harness Commands")
	fmt.Println("")
	fmt.Println("Usage:")
	fmt.Println("  doombox harness [-n] <subcommand>")
	fmt.Println("  doombox harness init [--agent codex|gemini|cloud] [PROJECT_PATH]")
	fmt.Println("  doombox harness status [--json] [PROJECT_PATH]")
	fmt.Println("  doombox harness score [--json] [PROJECT_PATH]")
	fmt.Println("  doombox harness report [--json] [--strict] [--min-score 0.70] [PROJECT_PATH]")
	fmt.Println("  doombox harness export-eval [--out FILE] [PROJECT_PATH]")
	fmt.Println("  doombox harness compare BASELINE_PATH CANDIDATE_PATH [--json] [--strict]")
	fmt.Println("  doombox harness flip --baseline BASELINE.json --candidate CANDIDATE.json [--json] [--strict]")
}

func (c *cli) runHarnessInit(args []string) error {
	fs := flag.NewFlagSet("harness init", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	agent := fs.String("agent", envOr("AGENT", "codex"), "agent: codex|gemini|cloud")
	if err := fs.Parse(args); err != nil {
		return err
	}

	projectPath := ""
	remaining := fs.Args()
	if len(remaining) > 0 {
		projectPath = strings.TrimSpace(remaining[0])
	}
	if projectPath == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		projectPath = cwd
	}

	absPath, err := filepath.Abs(projectPath)
	if err != nil {
		return err
	}
	if err := harness.Initialize(*agent, absPath); err != nil {
		return err
	}
	fmt.Printf("Harness initialized at %s/.doombox (agent=%s)\n", absPath, *agent)
	return nil
}

func (c *cli) runHarnessStatus(args []string) error {
	fs := flag.NewFlagSet("harness status", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	jsonOut := fs.Bool("json", false, "print JSON output")
	if err := fs.Parse(args); err != nil {
		return err
	}

	projectPath := ""
	remaining := fs.Args()
	if len(remaining) > 0 {
		projectPath = strings.TrimSpace(remaining[0])
	}
	if projectPath == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		projectPath = cwd
	}

	absPath, err := filepath.Abs(projectPath)
	if err != nil {
		return err
	}
	status, err := collectHarnessStatus(absPath)
	if err != nil {
		return err
	}
	if *jsonOut {
		b, err := json.MarshalIndent(status, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(b))
		return nil
	}

	fmt.Println("Doombox Harness Status")
	fmt.Println("======================")
	fmt.Printf("Project: %s\n", absPath)
	fmt.Printf("Events: %d\n", status.EventCount)
	fmt.Printf("Checkpoints: %d\n", status.CheckpointCount)
	fmt.Printf("Open TODOs: %d\n", status.OpenTodos)
	fmt.Printf("Risk block events: %d\n", status.BlockRiskCount)
	fmt.Printf("Risk justify events: %d\n", status.JustifyRiskCount)
	fmt.Printf("Last event: %s\n", status.LastEventType)
	return nil
}

func (c *cli) runHarnessScore(args []string) error {
	fs := flag.NewFlagSet("harness score", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	jsonOut := fs.Bool("json", false, "print JSON output")
	if err := fs.Parse(args); err != nil {
		return err
	}

	projectPath := ""
	remaining := fs.Args()
	if len(remaining) > 0 {
		projectPath = strings.TrimSpace(remaining[0])
	}
	if projectPath == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		projectPath = cwd
	}

	absPath, err := filepath.Abs(projectPath)
	if err != nil {
		return err
	}
	score, err := collectHarnessRubric(absPath)
	if err != nil {
		return err
	}
	if *jsonOut {
		b, err := json.MarshalIndent(score, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(b))
		return nil
	}

	fmt.Println("Doombox Harness Rubric")
	fmt.Println("======================")
	fmt.Printf("Project: %s\n", absPath)
	fmt.Printf("Score: %.2f\n", score.Score)
	fmt.Printf("Scope: %.2f\n", score.ScopeDiscipline)
	fmt.Printf("Test: %.2f\n", score.TestDiscipline)
	fmt.Printf("Safety: %.2f\n", score.Safety)
	fmt.Printf("Efficiency: %.2f\n", score.Efficiency)
	fmt.Printf("Events: %d\n", score.EventCount)
	fmt.Printf("Checkpoints: %d\n", score.CheckpointCount)
	return nil
}

func (c *cli) runHarnessFlip(args []string) error {
	fs := flag.NewFlagSet("harness flip", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	nonInteractive := fs.Bool("non-interactive", false, "disable interactive prompts")
	fs.BoolVar(nonInteractive, "n", false, "disable interactive prompts")
	baselinePath := fs.String("baseline", "", "path to baseline eval-run json")
	candidatePath := fs.String("candidate", "", "path to candidate eval-run json")
	jsonOut := fs.Bool("json", false, "print JSON output")
	strict := fs.Bool("strict", false, "exit non-zero if flip checks fail")
	maxRegressions := fs.Int("max-regressions", 0, "maximum allowed regressions")
	requirePositiveDelta := fs.Bool("require-positive-delta", false, "require positive average score delta")
	if err := fs.Parse(args); err != nil {
		return err
	}

	remaining := fs.Args()
	if *baselinePath == "" && len(remaining) > 0 {
		*baselinePath = strings.TrimSpace(remaining[0])
	}
	if *candidatePath == "" && len(remaining) > 1 {
		*candidatePath = strings.TrimSpace(remaining[1])
	}
	if *baselinePath == "" || *candidatePath == "" {
		if *nonInteractive || !isInteractiveTerminal() {
			return errors.New("harness flip requires --baseline and --candidate (or two positional file paths)")
		}
		if strings.TrimSpace(*baselinePath) == "" {
			in, err := promptInput("Baseline eval JSON path")
			if err != nil {
				return err
			}
			*baselinePath = strings.TrimSpace(in)
		}
		if strings.TrimSpace(*candidatePath) == "" {
			in, err := promptInput("Candidate eval JSON path")
			if err != nil {
				return err
			}
			*candidatePath = strings.TrimSpace(in)
		}
		if *baselinePath == "" || *candidatePath == "" {
			return errors.New("harness flip requires baseline and candidate paths")
		}
	}

	report, err := collectFlipReport(*baselinePath, *candidatePath)
	if err != nil {
		return err
	}

	if *jsonOut {
		b, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(b))
		return nil
	}

	gate := evaluateFlipGate(report, flipGateOptions{
		MaxRegressions:       *maxRegressions,
		RequirePositiveDelta: *requirePositiveDelta,
	})

	fmt.Println("Doombox Harness Flip Analysis")
	fmt.Println("=============================")
	fmt.Printf("Baseline: %s\n", *baselinePath)
	fmt.Printf("Candidate: %s\n", *candidatePath)
	fmt.Printf("Compared: %d\n", report.TotalCompared)
	fmt.Printf("Improved: %d\n", report.Improved)
	fmt.Printf("Regressed: %d\n", report.Regressed)
	fmt.Printf("Unchanged: %d\n", report.Unchanged)
	fmt.Printf("Avg score delta: %.2f\n", report.DeltaScoreAvg)
	fmt.Printf("Gate pass: %v\n", gate.Pass)
	if len(gate.Reasons) > 0 {
		fmt.Println("Gate reasons:")
		for _, reason := range gate.Reasons {
			fmt.Printf("- %s\n", reason)
		}
	}
	if *strict && !gate.Pass {
		return errors.New("harness flip strict checks failed")
	}
	return nil
}

func (c *cli) runHarnessReport(args []string) error {
	fs := flag.NewFlagSet("harness report", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	jsonOut := fs.Bool("json", false, "print JSON output")
	strict := fs.Bool("strict", false, "exit non-zero if health checks fail")
	minScore := fs.Float64("min-score", 0.70, "minimum rubric score required")
	maxOpenTodos := fs.Int("max-open-todos", 0, "maximum allowed open todos")
	maxBlockRisks := fs.Int("max-block-risks", 0, "maximum allowed block risk events")
	if err := fs.Parse(args); err != nil {
		return err
	}

	projectPath := ""
	remaining := fs.Args()
	if len(remaining) > 0 {
		projectPath = strings.TrimSpace(remaining[0])
	}
	if projectPath == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		projectPath = cwd
	}

	absPath, err := filepath.Abs(projectPath)
	if err != nil {
		return err
	}
	report, err := collectHarnessReport(absPath)
	if err != nil {
		return err
	}
	report.Health = evaluateHarnessHealth(report, harnessHealthOptions{
		MinScore:     *minScore,
		MaxOpenTodos: *maxOpenTodos,
		MaxBlockRisk: *maxBlockRisks,
	})

	if *jsonOut {
		b, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(b))
		return nil
	}

	fmt.Println("Doombox Harness Report")
	fmt.Println("======================")
	fmt.Printf("Project: %s\n", report.ProjectPath)
	fmt.Printf("Events file: %s\n", report.EventsPath)
	fmt.Printf("Checkpoints dir: %s\n", report.CheckpointsPath)
	fmt.Printf("TODO file: %s\n", report.TodoPath)
	fmt.Printf("Event count: %d\n", report.Status.EventCount)
	fmt.Printf("Checkpoint count: %d\n", report.Status.CheckpointCount)
	fmt.Printf("Open TODOs: %d\n", report.Status.OpenTodos)
	fmt.Printf("Rubric score: %.2f\n", report.Rubric.Score)
	fmt.Printf("Safety score: %.2f\n", report.Rubric.Safety)
	fmt.Printf("Health pass: %v\n", report.Health.Pass)
	if len(report.Health.Reasons) > 0 {
		fmt.Println("Health reasons:")
		for _, reason := range report.Health.Reasons {
			fmt.Printf("- %s\n", reason)
		}
	}
	if *strict && !report.Health.Pass {
		return errors.New("harness report strict checks failed")
	}
	return nil
}

func (c *cli) runHarnessExportEval(args []string) error {
	fs := flag.NewFlagSet("harness export-eval", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	outPath := fs.String("out", "", "write eval-run json to this file")
	runID := fs.String("id", "", "override eval run id")
	minScore := fs.Float64("min-score", 0.70, "minimum rubric score required for pass")
	maxOpenTodos := fs.Int("max-open-todos", 0, "maximum allowed open todos for pass")
	maxBlockRisks := fs.Int("max-block-risks", 0, "maximum allowed block risk events for pass")
	if err := fs.Parse(args); err != nil {
		return err
	}

	projectPath := ""
	remaining := fs.Args()
	if len(remaining) > 0 {
		projectPath = strings.TrimSpace(remaining[0])
	}
	if projectPath == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		projectPath = cwd
	}

	absPath, err := filepath.Abs(projectPath)
	if err != nil {
		return err
	}
	report, err := collectHarnessReport(absPath)
	if err != nil {
		return err
	}
	health := evaluateHarnessHealth(report, harnessHealthOptions{
		MinScore:     *minScore,
		MaxOpenTodos: *maxOpenTodos,
		MaxBlockRisk: *maxBlockRisks,
	})
	eval := evalRunFromHarnessReportWithHealth(report, health, *runID)

	b, err := json.MarshalIndent(eval, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(b))

	if strings.TrimSpace(*outPath) != "" {
		target := strings.TrimSpace(*outPath)
		if !filepath.IsAbs(target) {
			target = filepath.Join(absPath, target)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(target, append(b, '\n'), 0644); err != nil {
			return err
		}
		fmt.Printf("Exported eval run to %s\n", target)
	}
	return nil
}

func (c *cli) runHarnessCompare(args []string) error {
	fs := flag.NewFlagSet("harness compare", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	nonInteractive := fs.Bool("non-interactive", false, "disable interactive prompts")
	fs.BoolVar(nonInteractive, "n", false, "disable interactive prompts")
	jsonOut := fs.Bool("json", false, "print JSON output")
	strict := fs.Bool("strict", false, "exit non-zero if comparison checks fail")
	runID := fs.String("run-id", "current", "logical run id used to match baseline/candidate")
	minScore := fs.Float64("min-score", 0.70, "minimum rubric score required for pass")
	maxOpenTodos := fs.Int("max-open-todos", 0, "maximum allowed open todos for pass")
	maxBlockRisks := fs.Int("max-block-risks", 0, "maximum allowed block risk events for pass")
	maxRegressions := fs.Int("max-regressions", 0, "maximum allowed regressions")
	requirePositiveDelta := fs.Bool("require-positive-delta", false, "require positive average score delta")
	if err := fs.Parse(args); err != nil {
		return err
	}

	remaining := fs.Args()
	if len(remaining) < 2 {
		if *nonInteractive || !isInteractiveTerminal() {
			return errors.New("harness compare requires BASELINE_PATH and CANDIDATE_PATH")
		}
		baselineIn, err := promptInput("Baseline project path")
		if err != nil {
			return err
		}
		candidateIn, err := promptInput("Candidate project path")
		if err != nil {
			return err
		}
		remaining = []string{baselineIn, candidateIn}
	}
	if len(remaining) < 2 || strings.TrimSpace(remaining[0]) == "" || strings.TrimSpace(remaining[1]) == "" {
		return errors.New("harness compare requires BASELINE_PATH and CANDIDATE_PATH")
	}

	baselinePath, err := filepath.Abs(strings.TrimSpace(remaining[0]))
	if err != nil {
		return err
	}
	candidatePath, err := filepath.Abs(strings.TrimSpace(remaining[1]))
	if err != nil {
		return err
	}

	baselineReport, err := collectHarnessReport(baselinePath)
	if err != nil {
		return fmt.Errorf("collect baseline report: %w", err)
	}
	baselineReport.Health = evaluateHarnessHealth(baselineReport, harnessHealthOptions{
		MinScore:     *minScore,
		MaxOpenTodos: *maxOpenTodos,
		MaxBlockRisk: *maxBlockRisks,
	})
	candidateReport, err := collectHarnessReport(candidatePath)
	if err != nil {
		return fmt.Errorf("collect candidate report: %w", err)
	}
	candidateReport.Health = evaluateHarnessHealth(candidateReport, harnessHealthOptions{
		MinScore:     *minScore,
		MaxOpenTodos: *maxOpenTodos,
		MaxBlockRisk: *maxBlockRisks,
	})

	flip := harnessengine.AnalyzeFlips(
		[]harnessengine.EvalRun{evalRunFromHarnessReportWithHealth(baselineReport, baselineReport.Health, *runID)},
		[]harnessengine.EvalRun{evalRunFromHarnessReportWithHealth(candidateReport, candidateReport.Health, *runID)},
	)
	gate := evaluateFlipGate(flip, flipGateOptions{
		MaxRegressions:       *maxRegressions,
		RequirePositiveDelta: *requirePositiveDelta,
	})

	result := harnessCompare{
		Baseline:  baselineReport,
		Candidate: candidateReport,
		Flip:      flip,
		Gate:      gate,
	}

	if *jsonOut {
		b, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(b))
	} else {
		fmt.Println("Doombox Harness Compare")
		fmt.Println("=======================")
		fmt.Printf("Baseline: %s\n", baselinePath)
		fmt.Printf("Candidate: %s\n", candidatePath)
		fmt.Printf("Baseline score: %.2f (pass=%v)\n", baselineReport.Rubric.Score, baselineReport.Health.Pass)
		fmt.Printf("Candidate score: %.2f (pass=%v)\n", candidateReport.Rubric.Score, candidateReport.Health.Pass)
		fmt.Printf("Regressed: %d\n", flip.Regressed)
		fmt.Printf("Improved: %d\n", flip.Improved)
		fmt.Printf("Avg score delta: %.2f\n", flip.DeltaScoreAvg)
		fmt.Printf("Gate pass: %v\n", gate.Pass)
	}

	if *strict {
		if !baselineReport.Health.Pass {
			return errors.New("baseline health checks failed")
		}
		if !candidateReport.Health.Pass {
			return errors.New("candidate health checks failed")
		}
		if !gate.Pass {
			return errors.New("compare flip gate checks failed")
		}
	}
	return nil
}

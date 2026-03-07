package main

import (
	"bufio"
	"crypto/sha1"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/Goosebyteshq/doombox/harness"
	harnessadapters "github.com/Goosebyteshq/doombox/harness/adapters"
	harnessengine "github.com/Goosebyteshq/doombox/harness/engine"
)

type cli struct {
	composeBin  string
	composeArgs []string
}

//go:embed assets/*
var runtimeAssets embed.FS

func main() {
	c, err := newCLI()
	if err != nil {
		fatal(err)
	}

	if len(os.Args) < 2 {
		printRootHelp()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "open":
		err = c.runOpen(os.Args[2:])
	case "start", "connect":
		err = c.runOpen(os.Args[2:])
	case "list":
		err = c.runList(os.Args[2:])
	case "harness":
		err = c.runHarness(os.Args[2:])
	case "-h", "--help", "help":
		printRootHelp()
		return
	default:
		err = fmt.Errorf("unknown command %q", os.Args[1])
	}

	if err != nil {
		fatal(err)
	}
}

func newCLI() (*cli, error) {
	if path, err := exec.LookPath("docker-compose"); err == nil {
		return &cli{composeBin: path}, nil
	}
	if path, err := exec.LookPath("docker"); err == nil {
		return &cli{composeBin: path, composeArgs: []string{"compose"}}, nil
	}
	return nil, errors.New("docker compose not found (need docker-compose or docker compose)")
}

func printRootHelp() {
	fmt.Println("AI Sandbox CLI")
	fmt.Println("")
	fmt.Println("Usage:")
	fmt.Println("  doombox open [--agent claude|codex|gemini] [--detach] [PROJECT_PATH] [PROJECT_NAME]")
	fmt.Println("  doombox start [--agent claude|codex|gemini] [--detach] [PROJECT_PATH] [PROJECT_NAME]")
	fmt.Println("  doombox connect [--agent claude|codex|gemini] [--detach] [PROJECT_PATH] [PROJECT_NAME]")
	fmt.Println("  doombox list [--all]")
	fmt.Println("  doombox harness init [--agent codex|gemini|cloud] [PROJECT_PATH]")
	fmt.Println("  doombox harness status [PROJECT_PATH]")
	fmt.Println("  doombox harness score [PROJECT_PATH]")
	fmt.Println("  doombox harness report [--json] [--strict] [--min-score 0.70] [PROJECT_PATH]")
	fmt.Println("  doombox harness flip --baseline BASELINE.json --candidate CANDIDATE.json [--json] [--strict]")
}

func (c *cli) runOpen(args []string) error {
	fs := flag.NewFlagSet("open", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	agent := fs.String("agent", envOr("AGENT", "claude"), "agent: claude|codex|gemini")
	fs.StringVar(agent, "a", envOr("AGENT", "claude"), "agent: claude|codex|gemini")
	detach := fs.Bool("detach", false, "connect if running; otherwise start container and exit")
	fs.BoolVar(detach, "d", false, "connect if running; otherwise start container and exit")
	interactive := fs.Bool("interactive", true, "connect if running; otherwise start and connect")
	fs.BoolVar(interactive, "i", true, "connect if running; otherwise start and connect")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *detach {
		*interactive = false
	}

	absPath, projectName, err := resolveProjectPathAndName(fs.Args(), os.Stdin, os.Stdout)
	if err != nil {
		return err
	}

	agentCmd, err := commandForAgent(*agent, os.Getenv("AGENT_CMD"))
	if err != nil {
		return err
	}

	containerName := "ai-dev-" + projectName
	running, err := c.containerRunning(containerName)
	if err != nil {
		return err
	}
	if running {
		if !*interactive {
			fmt.Printf("Container already running for project %s.\n", projectName)
			fmt.Println("Container running in background.")
			fmt.Printf("Use `doombox open --agent %s %s` to connect.\n", *agent, absPath)
			return nil
		}
		fmt.Printf("Container already running for project %s. Connecting...\n", projectName)
		fmt.Printf("Connecting to %s for project: %s\n", *agent, projectName)
		return c.runWithHarness(*agent, absPath, func() error {
			return c.run("docker", []string{"exec", "-it", containerName, "bash", "-lc", agentCmd}, nil)
		})
	}

	fmt.Printf("No running container for project %s. Starting a new one...\n", projectName)
	return c.startOrReuseSession(*agent, absPath, projectName, *interactive)
}

func (c *cli) runList(args []string) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	all := fs.Bool("all", false, "include stopped containers")
	if err := fs.Parse(args); err != nil {
		return err
	}

	psArgs := []string{"ps", "--filter", "name=^ai-dev-", "--format", "{{.Names}}\t{{.Status}}\t{{.Image}}"}
	if *all {
		psArgs = append([]string{"ps", "-a"}, psArgs[2:]...)
	}
	out, err := c.capture("docker", psArgs, nil)
	if err != nil {
		return err
	}

	rows := parseDoomboxContainerRows(out)
	if len(rows) == 0 {
		if *all {
			fmt.Println("No doombox containers found.")
		} else {
			fmt.Println("No running doombox containers found.")
		}
		return nil
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Name < rows[j].Name })

	fmt.Println("NAME\tPROJECT\tSTATUS\tIMAGE")
	for _, row := range rows {
		fmt.Printf("%s\t%s\t%s\t%s\n", row.Name, row.Project, row.Status, row.Image)
	}
	return nil
}

func (c *cli) runHarness(args []string) error {
	if len(args) == 0 {
		printHarnessHelp()
		return nil
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
	fmt.Println("  doombox harness init [--agent codex|gemini|cloud] [PROJECT_PATH]")
	fmt.Println("  doombox harness status [--json] [PROJECT_PATH]")
	fmt.Println("  doombox harness score [--json] [PROJECT_PATH]")
	fmt.Println("  doombox harness report [--json] [--strict] [--min-score 0.70] [PROJECT_PATH]")
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
		return errors.New("harness flip requires --baseline and --candidate (or two positional file paths)")
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

func commandForAgent(agent, override string) (string, error) {
	if override != "" {
		return override, nil
	}
	adapter, err := harnessadapters.Lookup(agent)
	if err != nil {
		return "", fmt.Errorf("unsupported agent %q (expected claude, codex, gemini)", agent)
	}
	return adapter.DefaultCommand(), nil
}

type containerRow struct {
	Name    string
	Project string
	Status  string
	Image   string
}

func parseDoomboxContainerRows(out string) []containerRow {
	rows := []containerRow{}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 3 {
			continue
		}
		name := strings.TrimSpace(parts[0])
		if !strings.HasPrefix(name, "ai-dev-") {
			continue
		}
		rows = append(rows, containerRow{
			Name:    name,
			Project: projectNameFromContainerName(name),
			Status:  strings.TrimSpace(parts[1]),
			Image:   strings.TrimSpace(parts[2]),
		})
	}
	return rows
}

func projectNameFromContainerName(containerName string) string {
	return strings.TrimPrefix(containerName, "ai-dev-")
}

func resolveProjectPathAndName(pos []string, in io.Reader, out io.Writer) (string, string, error) {
	projectPath := ""
	if len(pos) >= 1 {
		projectPath = strings.TrimSpace(pos[0])
	}
	if projectPath == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", "", err
		}
		absCwd, err := filepath.Abs(cwd)
		if err != nil {
			return "", "", err
		}
		ok, err := confirmCurrentDirectoryMount(absCwd, in, out)
		if err != nil {
			return "", "", err
		}
		if !ok {
			return "", "", errors.New("aborted by user")
		}
		projectPath = absCwd
	}
	absPath, err := filepath.Abs(projectPath)
	if err != nil {
		return "", "", err
	}
	info, err := os.Stat(absPath)
	if err != nil || !info.IsDir() {
		return "", "", fmt.Errorf("project path does not exist: %s", absPath)
	}
	projectName := envOr("PROJECT_NAME", "")
	if len(pos) >= 2 {
		projectName = pos[1]
	}
	if projectName == "" {
		projectName = defaultProjectName(absPath)
	}
	return absPath, sanitizeProjectName(projectName), nil
}

func confirmCurrentDirectoryMount(absPath string, in io.Reader, out io.Writer) (bool, error) {
	fmt.Fprintln(out, "No project path provided.")
	fmt.Fprintf(out, "You are about to mount your current directory in YOLO mode:\n  %s\n", absPath)
	if home, err := os.UserHomeDir(); err == nil {
		if samePath(absPath, home) {
			fmt.Fprintln(out, "WARNING: This is your home directory.")
		}
	}
	fmt.Fprint(out, "Type 'yes' to continue: ")

	reader := bufio.NewReader(in)
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	return strings.EqualFold(strings.TrimSpace(line), "yes"), nil
}

func samePath(a, b string) bool {
	ca := filepath.Clean(a)
	cb := filepath.Clean(b)
	return ca == cb
}

func (c *cli) runWithHarness(agent, projectPath string, runFn func() error) error {
	return harness.RunWithSession(agent, projectPath, os.Stdout, runFn)
}

func (c *cli) startOrReuseSession(agent, absPath, projectName string, interactive bool) error {
	agentCmd, err := commandForAgent(agent, os.Getenv("AGENT_CMD"))
	if err != nil {
		return err
	}

	if _, err := c.capture("docker", []string{"info", "--format", "{{.ServerVersion}}"}, nil); err != nil {
		return errors.New("docker is not running")
	}

	runtimeDir, composeFile, cleanup, err := prepareRuntimeFiles()
	if err != nil {
		return err
	}
	defer cleanup()

	env := composeEnv(absPath, projectName, agent)
	env = append(env, "AI_SANDBOX_RUNTIME_DIR="+runtimeDir)
	containerName := "ai-dev-" + projectName
	composeProject := "ai-dev-" + projectName

	fmt.Println("AI Dev Docker Environment")
	fmt.Println("=========================")
	fmt.Printf("Project Path: %s\n", absPath)
	fmt.Printf("Project Name: %s\n", projectName)
	fmt.Printf("Agent: %s\n\n", agent)

	running, err := c.containerRunning(containerName)
	if err != nil {
		return err
	}

	if running {
		fmt.Println("Container already running, reusing existing container.")
	} else {
		fmt.Println("Building container...")
		if err := c.compose(composeFile, []string{"-p", composeProject, "build"}, env); err != nil {
			return err
		}

		fmt.Println("Starting container...")
		if err := c.compose(composeFile, []string{"-p", composeProject, "up", "-d"}, env); err != nil {
			return err
		}
		fmt.Println("Container started.")
	}

	fmt.Printf("\nProject mount:\n  %s -> /workspace/project\n\n", absPath)

	if interactive {
		fmt.Printf("Launching %s...\n\n", agent)
		sessionName := "doombox-" + projectName
		execArgs := []string{
			"-p", composeProject,
			"exec",
			"-e", "DOOMBOX_AGENT_CMD=" + agentCmd,
			"-e", "DOOMBOX_TMUX_SESSION=" + sessionName,
			"ai-dev", "bash", "-lc", "/opt/doombox/harness/scripts/launch_tmux.sh",
		}
		return c.runWithHarness(agent, absPath, func() error {
			return c.compose(composeFile, execArgs, env)
		})
	}

	fmt.Println("Container running in background.")
	fmt.Println("Connect with:")
	fmt.Printf("  doombox open --agent %s %s\n", agent, absPath)
	return nil
}

func composeEnv(projectPath, projectName, agent string) []string {
	env := os.Environ()
	env = append(env,
		"PROJECT_PATH="+projectPath,
		"PROJECT_NAME="+projectName,
		"AGENT="+agent,
		"AI_HOME_VOLUME=ai-dev-home-"+projectName,
	)
	return env
}

func (c *cli) compose(composeFile string, args []string, env []string) error {
	full := append([]string{}, c.composeArgs...)
	full = append(full, "-f", composeFile)
	full = append(full, args...)
	return c.run(c.composeBin, full, env)
}

func (c *cli) containerRunning(containerName string) (bool, error) {
	out, err := c.capture("docker", []string{"ps", "--filter", "name=^" + containerName + "$", "--format", "{{.Names}}"}, nil)
	if err != nil {
		return false, err
	}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if strings.TrimSpace(line) == containerName {
			return true, nil
		}
	}
	return false, nil
}

func (c *cli) run(bin string, args []string, env []string) error {
	cmd := exec.Command(bin, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if env != nil {
		cmd.Env = env
	}
	return cmd.Run()
}

func (c *cli) capture(bin string, args []string, env []string) (string, error) {
	cmd := exec.Command(bin, args...)
	if env != nil {
		cmd.Env = env
	}
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func defaultProjectName(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	base := filepath.Base(abs)
	base = sanitizeProjectName(base)
	if base == "" {
		base = "project"
	}
	h := sha1.Sum([]byte(abs))
	short := hex.EncodeToString(h[:])[:6]
	return fmt.Sprintf("%s-%s", base, short)
}

var nonProjectChars = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

func sanitizeProjectName(name string) string {
	v := strings.TrimSpace(name)
	v = nonProjectChars.ReplaceAllString(v, "-")
	v = strings.Trim(v, "-_.")
	v = strings.ToLower(v)
	if v == "" {
		return "project"
	}
	if len(v) > 48 {
		v = v[:48]
	}
	return v
}

func envOr(key, fallback string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	return v
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(1)
}

func prepareRuntimeFiles() (runtimeDir string, composeFile string, cleanup func(), err error) {
	runtimeDir, err = os.MkdirTemp("", "doombox-runtime-*")
	if err != nil {
		return "", "", nil, err
	}

	cleanup = func() {
		_ = os.RemoveAll(runtimeDir)
	}

	files := []struct {
		src  string
		dst  string
		mode fs.FileMode
	}{
		{src: "assets/docker-compose.yml", dst: "docker-compose.yml", mode: 0644},
		{src: "assets/Dockerfile", dst: "Dockerfile", mode: 0644},
		{src: "assets/entrypoint.sh", dst: "entrypoint.sh", mode: 0755},
	}

	for _, f := range files {
		data, readErr := runtimeAssets.ReadFile(f.src)
		if readErr != nil {
			cleanup()
			return "", "", nil, fmt.Errorf("read embedded asset %s: %w", f.src, readErr)
		}
		target := filepath.Join(runtimeDir, f.dst)
		if writeErr := os.WriteFile(target, data, f.mode); writeErr != nil {
			cleanup()
			return "", "", nil, fmt.Errorf("write runtime asset %s: %w", f.dst, writeErr)
		}
	}

	if err := harness.WriteScaffold(filepath.Join(runtimeDir, "harness")); err != nil {
		cleanup()
		return "", "", nil, fmt.Errorf("write harness scaffold: %w", err)
	}

	return runtimeDir, filepath.Join(runtimeDir, "docker-compose.yml"), cleanup, nil
}

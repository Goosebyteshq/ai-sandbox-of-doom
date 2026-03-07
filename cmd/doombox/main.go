package main

import (
	"bufio"
	"crypto/sha1"
	"embed"
	"encoding/hex"
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

func commandForAgent(agent, override string) (string, error) {
	if override != "" {
		return override, nil
	}
	switch agent {
	case "claude":
		return "claude --dangerously-skip-permissions", nil
	case "codex":
		return "codex --sandbox danger-full-access --ask-for-approval never", nil
	case "gemini":
		return "gemini --yolo", nil
	default:
		return "", fmt.Errorf("unsupported agent %q (expected claude, codex, gemini)", agent)
	}
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
		return c.runWithHarness(agent, absPath, func() error {
			return c.compose(composeFile, []string{"-p", composeProject, "exec", "ai-dev", "bash", "-lc", agentCmd}, env)
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

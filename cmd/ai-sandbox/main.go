package main

import (
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

type cli struct {
	composeBin  string
	composeArgs []string
}

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
	case "start":
		err = c.runStart(os.Args[2:])
	case "connect":
		err = c.runConnect(os.Args[2:])
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
	fmt.Println("  ai-sandbox start [--agent claude|codex|gemini] [--detach] [PROJECT_PATH] [PROJECT_NAME]")
	fmt.Println("  ai-sandbox connect [--agent claude|codex|gemini] [PROJECT_NAME]")
}

func (c *cli) runStart(args []string) error {
	fs := flag.NewFlagSet("start", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	agent := fs.String("agent", envOr("AGENT", "claude"), "agent: claude|codex|gemini")
	fs.StringVar(agent, "a", envOr("AGENT", "claude"), "agent: claude|codex|gemini")
	detach := fs.Bool("detach", false, "start container and exit")
	fs.BoolVar(detach, "d", false, "start container and exit")
	interactive := fs.Bool("interactive", true, "start and connect")
	fs.BoolVar(interactive, "i", true, "start and connect")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *detach {
		*interactive = false
	}

	pos := fs.Args()
	projectPath := envOr("PROJECT_PATH", "")
	if len(pos) >= 1 {
		projectPath = pos[0]
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
	info, err := os.Stat(absPath)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("project path does not exist: %s", absPath)
	}

	projectName := envOr("PROJECT_NAME", "")
	if len(pos) >= 2 {
		projectName = pos[1]
	}
	if projectName == "" {
		projectName = defaultProjectName(absPath)
	}
	projectName = sanitizeProjectName(projectName)

	agentCmd, err := commandForAgent(*agent, os.Getenv("AGENT_CMD"))
	if err != nil {
		return err
	}

	if err := c.run("docker", []string{"info"}, nil); err != nil {
		return errors.New("docker is not running")
	}

	env := composeEnv(absPath, projectName, *agent)
	containerName := "ai-dev-" + projectName
	composeProject := "ai-dev-" + projectName

	fmt.Println("AI Dev Docker Environment")
	fmt.Println("=========================")
	fmt.Printf("Project Path: %s\n", absPath)
	fmt.Printf("Project Name: %s\n", projectName)
	fmt.Printf("Agent: %s\n\n", *agent)

	running, err := c.containerRunning(containerName)
	if err != nil {
		return err
	}

	if running {
		fmt.Println("Container already running, reusing existing container.")
	} else {
		fmt.Println("Building container...")
		if err := c.compose([]string{"-p", composeProject, "build"}, env); err != nil {
			return err
		}

		fmt.Println("Starting container...")
		if err := c.compose([]string{"-p", composeProject, "up", "-d"}, env); err != nil {
			return err
		}
		fmt.Println("Container started.")
	}

	fmt.Printf("\nProject mount:\n  %s -> /workspace/project\n\n", absPath)

	if *interactive {
		fmt.Printf("Launching %s...\n\n", *agent)
		return c.compose([]string{"-p", composeProject, "exec", "ai-dev", "bash", "-lc", agentCmd}, env)
	}

	fmt.Println("Container running in background.")
	fmt.Println("Connect with:")
	fmt.Printf("  ./connect.sh --agent %s %s\n", *agent, projectName)
	return nil
}

func (c *cli) runConnect(args []string) error {
	fs := flag.NewFlagSet("connect", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	agent := fs.String("agent", envOr("AGENT", "claude"), "agent: claude|codex|gemini")
	fs.StringVar(agent, "a", envOr("AGENT", "claude"), "agent: claude|codex|gemini")
	if err := fs.Parse(args); err != nil {
		return err
	}

	var projectName string
	if len(fs.Args()) >= 1 {
		projectName = fs.Args()[0]
	} else {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		projectName = defaultProjectName(cwd)
	}
	projectName = sanitizeProjectName(projectName)

	agentCmd, err := commandForAgent(*agent, os.Getenv("AGENT_CMD"))
	if err != nil {
		return err
	}

	containerName := "ai-dev-" + projectName
	fmt.Printf("Connecting to %s for project: %s\n", *agent, projectName)

	running, err := c.containerRunning(containerName)
	if err != nil {
		return err
	}
	if !running {
		return fmt.Errorf("no running container found for project: %s", projectName)
	}

	return c.run("docker", []string{"exec", "-it", containerName, "bash", "-lc", agentCmd}, nil)
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

func (c *cli) compose(args []string, env []string) error {
	full := append([]string{}, c.composeArgs...)
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

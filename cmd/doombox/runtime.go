package main

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Goosebyteshq/doombox/harness"
)

//go:embed assets/*
var runtimeAssets embed.FS

func (c *cli) runWithHarness(agent, projectPath string, runFn func() error) error {
	return harness.RunWithSession(agent, projectPath, os.Stdout, runFn)
}

func (c *cli) startOrReuseSession(agent, absPath, projectName string, interactive bool, layout string) error {
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
			"-e", "DOOMBOX_LAYOUT=" + layout,
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

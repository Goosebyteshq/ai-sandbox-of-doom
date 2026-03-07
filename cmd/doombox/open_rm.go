package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func (c *cli) runOpen(args []string) error {
	fs := flag.NewFlagSet("open", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	nonInteractive := fs.Bool("non-interactive", false, "disable interactive prompts")
	fs.BoolVar(nonInteractive, "n", false, "disable interactive prompts")
	agent := fs.String("agent", envOr("AGENT", "claude"), "agent: claude|codex|gemini")
	fs.StringVar(agent, "a", envOr("AGENT", "claude"), "agent: claude|codex|gemini")
	imageRef := fs.String("image", envOr("DOOMBOX_IMAGE", defaultDoomboxImage), "container image reference")
	forceBuild := fs.Bool("build", false, "force local container build instead of pulling prebuilt image")
	fs.BoolVar(forceBuild, "b", false, "force local container build instead of pulling prebuilt image")
	layout := fs.String("layout", envOr("DOOMBOX_LAYOUT", "windows"), "tmux layout: windows|compact")
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
	normalizedLayout, err := normalizeTmuxLayout(*layout)
	if err != nil {
		return err
	}

	pos := fs.Args()
	if len(pos) == 0 {
		if *nonInteractive || !isInteractiveTerminal() {
			return errors.New("open requires PROJECT_PATH in non-interactive mode")
		}
		projectPath, err := c.promptOpenProjectPath()
		if err != nil {
			return err
		}
		pos = append(pos, projectPath)
	}

	absPath, projectName, err := resolveProjectPathAndName(pos, os.Stdin, os.Stdout)
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
		sessionName := "doombox-" + projectName
		agentCmd, err := commandForAgent(*agent, os.Getenv("AGENT_CMD"))
		if err != nil {
			return err
		}
		return c.runWithHarness(*agent, absPath, func() error {
			return c.run("docker", []string{
				"exec", "-it",
				"-e", "DOOMBOX_AGENT_CMD=" + agentCmd,
				"-e", "DOOMBOX_TMUX_SESSION=" + sessionName,
				"-e", "DOOMBOX_LAYOUT=" + normalizedLayout,
				containerName,
				"bash", "-lc", "/opt/doombox/harness/scripts/launch_tmux.sh",
			}, nil)
		})
	}

	fmt.Printf("No running container for project %s. Starting a new one...\n", projectName)
	return c.startOrReuseSession(*agent, absPath, projectName, *interactive, normalizedLayout, *imageRef, *forceBuild)
}

func (c *cli) promptOpenProjectPath() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	absCwd, err := filepath.Abs(cwd)
	if err != nil {
		return "", err
	}

	rows, err := c.listDoomboxContainerRows(false)
	if err != nil {
		return "", err
	}

	options := []string{fmt.Sprintf("Open current directory (%s)", absCwd)}
	lookup := map[int]string{
		0: "cwd",
	}
	for _, row := range rows {
		idx := len(options)
		options = append(options, fmt.Sprintf("Attach running project (%s)", row.Project))
		lookup[idx] = row.Name
	}
	manualIdx := len(options)
	options = append(options, "Enter project path")
	cancelIdx := len(options)
	options = append(options, "Cancel")

	choice, err := promptSelect("Choose project to open", options)
	if err != nil {
		return "", err
	}

	if choice == cancelIdx {
		return "", errors.New("aborted by user")
	}
	if choice == manualIdx {
		projectPath, err := promptInput("Project path")
		if err != nil {
			return "", err
		}
		projectPath = strings.TrimSpace(projectPath)
		if projectPath == "" {
			return "", errors.New("project path is required")
		}
		return projectPath, nil
	}
	if lookup[choice] == "cwd" {
		if home, err := os.UserHomeDir(); err == nil && samePath(absCwd, home) {
			ok, err := promptYesNo("Current directory is your home directory. Continue?")
			if err != nil {
				return "", err
			}
			if !ok {
				return "", errors.New("aborted by user")
			}
		}
		return absCwd, nil
	}

	containerName := lookup[choice]
	mountPath, err := c.containerProjectMountPath(containerName)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(mountPath) == "" {
		return "", fmt.Errorf("could not resolve project path for running container %s", containerName)
	}
	return mountPath, nil
}

func (c *cli) runList(args []string) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	all := fs.Bool("all", false, "include stopped containers")
	if err := fs.Parse(args); err != nil {
		return err
	}
	rows, err := c.listDoomboxContainerRows(*all)
	if err != nil {
		return err
	}
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

func (c *cli) listDoomboxContainerRows(all bool) ([]containerRow, error) {
	psArgs := []string{"ps", "--filter", "name=^ai-dev-", "--format", "{{.Names}}\t{{.Status}}\t{{.Image}}"}
	if all {
		psArgs = append([]string{"ps", "-a"}, psArgs[2:]...)
	}
	out, err := c.capture("docker", psArgs, nil)
	if err != nil {
		return nil, err
	}
	return parseDoomboxContainerRows(out), nil
}

func (c *cli) runRM(args []string) error {
	fs := flag.NewFlagSet("rm", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	nonInteractive := fs.Bool("non-interactive", false, "disable interactive prompts")
	fs.BoolVar(nonInteractive, "n", false, "disable interactive prompts")
	all := fs.Bool("all", false, "remove all doombox containers (running or stopped)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	containerNames, err := c.listDoomboxContainerNames(true)
	if err != nil {
		return err
	}
	if len(containerNames) == 0 {
		fmt.Println("No doombox containers found.")
		return nil
	}

	existing := map[string]struct{}{}
	for _, name := range containerNames {
		existing[name] = struct{}{}
	}

	targets := []string{}
	if *all {
		targets = append(targets, containerNames...)
	} else {
		rawTargets := fs.Args()
		if len(rawTargets) == 0 {
			if *nonInteractive || !isInteractiveTerminal() {
				cwd, err := os.Getwd()
				if err != nil {
					return err
				}
				containerName, err := doomboxContainerNameFromTarget(cwd)
				if err != nil {
					return err
				}
				rawTargets = []string{cwd}
				targets = append(targets, containerName)
				fmt.Printf("No target provided. Using current project container target from %s.\n", cwd)
			} else {
				selected, err := promptMultiSelect("Select doombox container(s) to remove", containerNames)
				if err != nil {
					return err
				}
				if len(selected) == 0 {
					fmt.Println("No containers selected. Nothing removed.")
					return nil
				}
				for _, idx := range selected {
					targets = append(targets, containerNames[idx])
				}
			}
		} else {
			for _, raw := range rawTargets {
				containerName, err := doomboxContainerNameFromTarget(raw)
				if err != nil {
					return err
				}
				targets = append(targets, containerName)
			}
		}
	}

	removeList := []string{}
	missing := []string{}
	seen := map[string]struct{}{}
	for _, target := range targets {
		if _, ok := seen[target]; ok {
			continue
		}
		seen[target] = struct{}{}
		if _, ok := existing[target]; ok {
			removeList = append(removeList, target)
		} else {
			missing = append(missing, target)
		}
	}

	if len(removeList) == 0 {
		if len(missing) > 0 {
			return fmt.Errorf("no matching doombox containers found for: %s (use `doombox list --all`)", strings.Join(missing, ", "))
		}
		fmt.Println("No matching doombox containers found.")
		return nil
	}

	if !*nonInteractive && len(fs.Args()) == 0 && !*all && isInteractiveTerminal() {
		ok, err := promptYesNo(fmt.Sprintf("Remove %d container(s)?", len(removeList)))
		if err != nil {
			return err
		}
		if !ok {
			fmt.Println("Aborted.")
			return nil
		}
	}

	fmt.Printf("Removing %d doombox container(s): %s\n", len(removeList), strings.Join(removeList, ", "))
	for _, name := range removeList {
		if err := c.run("docker", []string{"rm", "-f", name}, nil); err != nil {
			return err
		}
	}
	if len(missing) > 0 {
		fmt.Printf("Skipped missing containers: %s\n", strings.Join(missing, ", "))
	}
	return nil
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

func (c *cli) listDoomboxContainerNames(all bool) ([]string, error) {
	args := []string{"ps", "--filter", "name=^ai-dev-", "--format", "{{.Names}}"}
	if all {
		args = []string{"ps", "-a", "--filter", "name=^ai-dev-", "--format", "{{.Names}}"}
	}
	out, err := c.capture("docker", args, nil)
	if err != nil {
		return nil, err
	}
	names := []string{}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		name := strings.TrimSpace(line)
		if name == "" || !strings.HasPrefix(name, "ai-dev-") {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

func (c *cli) containerProjectMountPath(containerName string) (string, error) {
	out, err := c.capture("docker", []string{
		"inspect",
		"--format",
		"{{ range .Mounts }}{{ if eq .Destination \"/workspace/project\" }}{{ .Source }}{{ end }}{{ end }}",
		containerName,
	}, nil)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func doomboxContainerNameFromTarget(target string) (string, error) {
	trimmed := strings.TrimSpace(target)
	if trimmed == "" {
		return "", errors.New("empty rm target")
	}
	if strings.HasPrefix(trimmed, "ai-dev-") {
		return trimmed, nil
	}
	if info, err := os.Stat(trimmed); err == nil && info.IsDir() {
		abs, err := filepath.Abs(trimmed)
		if err != nil {
			return "", err
		}
		return "ai-dev-" + defaultProjectName(abs), nil
	}
	return "ai-dev-" + sanitizeProjectName(trimmed), nil
}

func projectNameFromContainerName(containerName string) string {
	return strings.TrimPrefix(containerName, "ai-dev-")
}

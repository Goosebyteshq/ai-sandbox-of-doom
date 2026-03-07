package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
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
		if isInteractiveTerminal() {
			if err := c.runRootMenu(); err != nil {
				fatal(err)
			}
			return
		}
		printRootHelp()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "open":
		err = c.runOpen(os.Args[2:])
	case "rm":
		err = c.runRM(os.Args[2:])
	case "list", "ls":
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
	fmt.Println("  doombox open [-n] [--agent claude|codex|gemini] [--image IMAGE] [--build] [--layout windows|compact] [--detach] [PROJECT_PATH] [PROJECT_NAME]")
	fmt.Println("  doombox rm [-n] [--all] [PROJECT_NAME|CONTAINER_NAME|PROJECT_PATH ...]")
	fmt.Println("  doombox list|ls [--all]")
	fmt.Println("  doombox harness [-n] <subcommand>")
	fmt.Println("  doombox harness init [--agent codex|gemini|cloud] [PROJECT_PATH]")
	fmt.Println("  doombox harness status [PROJECT_PATH]")
	fmt.Println("  doombox harness score [PROJECT_PATH]")
	fmt.Println("  doombox harness report [--json] [--strict] [--min-score 0.70] [PROJECT_PATH]")
	fmt.Println("  doombox harness export-eval [--out FILE] [PROJECT_PATH]")
	fmt.Println("  doombox harness compare BASELINE_PATH CANDIDATE_PATH [--json] [--strict]")
	fmt.Println("  doombox harness flip --baseline BASELINE.json --candidate CANDIDATE.json [--json] [--strict]")
}

func (c *cli) runRootMenu() error {
	options := []string{
		"open",
		"rm",
		"list",
		"harness",
		"help",
		"exit",
	}
	idx, err := promptSelect("Choose a doombox command", options)
	if err != nil {
		return err
	}
	switch options[idx] {
	case "open":
		return c.runOpen([]string{})
	case "rm":
		return c.runRM([]string{})
	case "list":
		return c.runList([]string{})
	case "harness":
		return c.runHarness([]string{})
	case "help":
		printRootHelp()
		return nil
	default:
		return nil
	}
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(1)
}

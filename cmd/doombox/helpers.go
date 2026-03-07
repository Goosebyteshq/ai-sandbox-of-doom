package main

import (
	"bufio"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	harnessadapters "github.com/Goosebyteshq/doombox/harness/adapters"
)

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

func normalizeTmuxLayout(layout string) (string, error) {
	v := strings.ToLower(strings.TrimSpace(layout))
	if v == "" {
		v = "windows"
	}
	switch v {
	case "windows", "compact":
		return v, nil
	default:
		return "", fmt.Errorf("unsupported --layout %q (expected windows|compact)", layout)
	}
}

func isInteractiveTerminal() bool {
	in, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	out, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (in.Mode()&os.ModeCharDevice) != 0 && (out.Mode()&os.ModeCharDevice) != 0
}

func promptInput(label string) (string, error) {
	fmt.Printf("%s: ", strings.TrimSpace(label))
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func promptYesNo(label string) (bool, error) {
	answer, err := promptInput(label + " [y/N]")
	if err != nil {
		return false, err
	}
	answer = strings.ToLower(strings.TrimSpace(answer))
	return answer == "y" || answer == "yes", nil
}

func promptSelect(label string, options []string) (int, error) {
	if len(options) == 0 {
		return 0, errors.New("no options available")
	}
	fmt.Println(label + ":")
	for i, option := range options {
		fmt.Printf("  %d. %s\n", i+1, option)
	}
	choice, err := promptInput("Select option number")
	if err != nil {
		return 0, err
	}
	indices, err := parseSelectionIndices(choice, len(options), false)
	if err != nil {
		return 0, err
	}
	return indices[0], nil
}

func promptMultiSelect(label string, options []string) ([]int, error) {
	if len(options) == 0 {
		return nil, nil
	}
	fmt.Println(label + ":")
	for i, option := range options {
		fmt.Printf("  %d. %s\n", i+1, option)
	}
	choice, err := promptInput("Select option numbers (comma-separated)")
	if err != nil {
		return nil, err
	}
	choice = strings.TrimSpace(choice)
	if choice == "" {
		return nil, nil
	}
	return parseSelectionIndices(choice, len(options), true)
}

func parseSelectionIndices(input string, max int, multi bool) ([]int, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return nil, errors.New("selection is required")
	}

	rawParts := []string{trimmed}
	if multi {
		rawParts = strings.Split(trimmed, ",")
	}
	seen := map[int]struct{}{}
	out := []int{}
	for _, part := range rawParts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		n, err := strconv.Atoi(part)
		if err != nil {
			return nil, fmt.Errorf("invalid selection %q", part)
		}
		if n < 1 || n > max {
			return nil, fmt.Errorf("selection %d out of range (1-%d)", n, max)
		}
		idx := n - 1
		if _, ok := seen[idx]; ok {
			continue
		}
		seen[idx] = struct{}{}
		out = append(out, idx)
	}
	if len(out) == 0 {
		return nil, errors.New("no valid selection provided")
	}
	if !multi && len(out) != 1 {
		return nil, errors.New("select exactly one option")
	}
	return out, nil
}

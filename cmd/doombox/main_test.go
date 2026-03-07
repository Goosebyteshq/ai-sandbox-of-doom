package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSanitizeProjectName(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: " My Project ", want: "my-project"},
		{in: "UPPER_lower.123", want: "upper_lower.123"},
		{in: "***", want: "project"},
		{in: "a/b/c", want: "a-b-c"},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := sanitizeProjectName(tt.in); got != tt.want {
				t.Fatalf("sanitizeProjectName(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestSanitizeProjectNameTruncates(t *testing.T) {
	in := strings.Repeat("a", 80)
	got := sanitizeProjectName(in)
	if len(got) != 48 {
		t.Fatalf("expected length 48, got %d", len(got))
	}
}

func TestDefaultProjectNameHasStableSuffix(t *testing.T) {
	path := filepath.Join("/tmp", "Example-Project")
	got := defaultProjectName(path)

	parts := strings.Split(got, "-")
	if len(parts) < 2 {
		t.Fatalf("expected hashed name format, got %q", got)
	}
	last := parts[len(parts)-1]
	if len(last) != 6 {
		t.Fatalf("expected 6-char suffix, got %q", last)
	}
	if got2 := defaultProjectName(path); got2 != got {
		t.Fatalf("expected deterministic value, got %q then %q", got, got2)
	}
}

func TestCommandForAgent(t *testing.T) {
	tests := []struct {
		agent   string
		want    string
		wantErr bool
	}{
		{agent: "claude", want: "claude --dangerously-skip-permissions"},
		{agent: "codex", want: "codex --sandbox danger-full-access --ask-for-approval never"},
		{agent: "gemini", want: "gemini --yolo"},
		{agent: "unknown", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.agent, func(t *testing.T) {
			got, err := commandForAgent(tt.agent, "")
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q", tt.agent)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tt.agent, err)
			}
			if got != tt.want {
				t.Fatalf("commandForAgent(%q) = %q, want %q", tt.agent, got, tt.want)
			}
		})
	}
}

func TestCommandForAgentOverride(t *testing.T) {
	got, err := commandForAgent("claude", "custom command")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "custom command" {
		t.Fatalf("expected override command, got %q", got)
	}
}

func TestComposeEnvContainsIsolationVars(t *testing.T) {
	env := composeEnv("/repo", "proj-1", "codex")
	joined := strings.Join(env, "\n")
	for _, want := range []string{
		"PROJECT_PATH=/repo",
		"PROJECT_NAME=proj-1",
		"AGENT=codex",
		"AI_HOME_VOLUME=ai-dev-home-proj-1",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("compose env missing %q", want)
		}
	}
}

func TestEnvOr(t *testing.T) {
	key := "AI_SANDBOX_TEST_ENV"
	_ = os.Unsetenv(key)
	if got := envOr(key, "fallback"); got != "fallback" {
		t.Fatalf("expected fallback, got %q", got)
	}
	if err := os.Setenv(key, "value"); err != nil {
		t.Fatalf("setenv: %v", err)
	}
	t.Cleanup(func() { _ = os.Unsetenv(key) })
	if got := envOr(key, "fallback"); got != "value" {
		t.Fatalf("expected env value, got %q", got)
	}
}

func TestResolveProjectPathAndNameRequiresPath(t *testing.T) {
	_, _, err := resolveProjectPathAndName(nil, strings.NewReader("no\n"), &strings.Builder{})
	if err == nil {
		t.Fatal("expected error when project path is missing")
	}
	if !strings.Contains(err.Error(), "aborted by user") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveProjectPathAndName(t *testing.T) {
	projectDir := t.TempDir()
	absDir, err := filepath.Abs(projectDir)
	if err != nil {
		t.Fatalf("abs: %v", err)
	}

	name := "Custom Name"
	gotPath, gotName, err := resolveProjectPathAndName([]string{projectDir, name}, strings.NewReader(""), &strings.Builder{})
	if err != nil {
		t.Fatalf("resolveProjectPathAndName returned error: %v", err)
	}
	if gotPath != absDir {
		t.Fatalf("got path %q, want %q", gotPath, absDir)
	}
	if gotName != "custom-name" {
		t.Fatalf("got project name %q, want custom-name", gotName)
	}
}

func TestConfirmCurrentDirectoryMount(t *testing.T) {
	out := &strings.Builder{}
	ok, err := confirmCurrentDirectoryMount("/tmp/project", strings.NewReader("yes\n"), out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected confirmation to succeed")
	}
	if !strings.Contains(out.String(), "/tmp/project") {
		t.Fatalf("expected output to include path, got %q", out.String())
	}
}

func TestConfirmCurrentDirectoryMountRejects(t *testing.T) {
	ok, err := confirmCurrentDirectoryMount("/tmp/project", strings.NewReader("no\n"), &strings.Builder{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected confirmation to be rejected")
	}
}

func TestParseDoomboxContainerRows(t *testing.T) {
	out := strings.Join([]string{
		"ai-dev-alpha\tUp 2 minutes\tyolo-ai-dev-mode-ai-dev",
		"unrelated\tUp 10 minutes\tnginx:latest",
		"ai-dev-beta\tExited (0) 1 hour ago\tyolo-ai-dev-mode-ai-dev",
	}, "\n")

	rows := parseDoomboxContainerRows(out)
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0].Project != "alpha" {
		t.Fatalf("unexpected first project: %q", rows[0].Project)
	}
	if rows[1].Project != "beta" {
		t.Fatalf("unexpected second project: %q", rows[1].Project)
	}
}

func TestCollectHarnessStatus(t *testing.T) {
	projectDir := t.TempDir()
	doomboxDir := filepath.Join(projectDir, ".doombox")
	if err := os.MkdirAll(filepath.Join(doomboxDir, "checkpoints"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	events := strings.Join([]string{
		`{"event_type":"session_start","risk_classification":"safe"}`,
		`{"event_type":"tool_invocation","risk_classification":"justify"}`,
		`{"event_type":"gate_decision","risk_classification":"block"}`,
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(doomboxDir, "events.jsonl"), []byte(events), 0644); err != nil {
		t.Fatalf("write events: %v", err)
	}
	if err := os.WriteFile(filepath.Join(doomboxDir, "todo.json"), []byte(`{"items":[{"status":"open"},{"status":"closed"},{"status":"open"}]}`), 0644); err != nil {
		t.Fatalf("write todo: %v", err)
	}
	if err := os.WriteFile(filepath.Join(doomboxDir, "checkpoints", "cp-1.json"), []byte(`{}`), 0644); err != nil {
		t.Fatalf("write checkpoint: %v", err)
	}

	status, err := collectHarnessStatus(projectDir)
	if err != nil {
		t.Fatalf("collectHarnessStatus: %v", err)
	}
	if status.EventCount != 3 {
		t.Fatalf("expected 3 events, got %d", status.EventCount)
	}
	if status.OpenTodos != 2 {
		t.Fatalf("expected 2 open todos, got %d", status.OpenTodos)
	}
	if status.CheckpointCount != 1 {
		t.Fatalf("expected 1 checkpoint, got %d", status.CheckpointCount)
	}
	if status.JustifyRiskCount != 1 || status.BlockRiskCount != 1 {
		t.Fatalf("unexpected risk counts: justify=%d block=%d", status.JustifyRiskCount, status.BlockRiskCount)
	}
	if status.LastEventType != "gate_decision" {
		t.Fatalf("expected last event gate_decision, got %q", status.LastEventType)
	}
}

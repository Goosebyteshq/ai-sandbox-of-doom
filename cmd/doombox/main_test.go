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

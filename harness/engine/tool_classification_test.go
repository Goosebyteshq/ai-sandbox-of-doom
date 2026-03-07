package engine

import (
	"os"
	"path/filepath"
	"testing"
)

func TestToolClassifierBlockedCommand(t *testing.T) {
	classifier := NewDefaultToolClassifier()
	got := classifier.Classify(ToolInvocation{
		Command: "rm -rf /tmp/something",
	})
	if got.Risk != "block" {
		t.Fatalf("expected block risk, got %q", got.Risk)
	}
}

func TestToolClassifierJustifyCommand(t *testing.T) {
	classifier := NewDefaultToolClassifier()
	got := classifier.Classify(ToolInvocation{
		Command: "git push --force origin main",
	})
	if got.Risk != "justify" {
		t.Fatalf("expected justify risk, got %q", got.Risk)
	}
}

func TestToolClassifierBlockedPath(t *testing.T) {
	classifier := NewDefaultToolClassifier()
	got := classifier.Classify(ToolInvocation{
		Command: "cat /etc/hosts",
		Files:   []string{"/etc/hosts"},
	})
	if got.Risk != "block" {
		t.Fatalf("expected block risk, got %q", got.Risk)
	}
}

func TestToolClassifierSafeDefault(t *testing.T) {
	classifier := NewDefaultToolClassifier()
	got := classifier.Classify(ToolInvocation{
		Command: "go test ./...",
		Files:   []string{"harness/session.go"},
	})
	if got.Risk != "safe" {
		t.Fatalf("expected safe risk, got %q", got.Risk)
	}
	if got.Rule != "default_safe" {
		t.Fatalf("expected default_safe rule, got %q", got.Rule)
	}
}

func TestToolClassifierFromPolicyFile(t *testing.T) {
	dir := t.TempDir()
	policyPath := filepath.Join(dir, "policy.json")
	err := os.WriteFile(policyPath, []byte(`{
  "sensitive_paths": ["secrets/"],
  "risky_paths": ["infra/"],
  "blocked_command_prefixes": ["danger-tool"],
  "justify_command_prefixes": ["risky-tool"]
}`), 0644)
	if err != nil {
		t.Fatalf("write policy file: %v", err)
	}

	classifier := ToolClassifierFromPolicyFile(policyPath)

	blocked := classifier.Classify(ToolInvocation{
		Command: "danger-tool run",
	})
	if blocked.Risk != "block" {
		t.Fatalf("expected block risk from policy command, got %q", blocked.Risk)
	}

	justify := classifier.Classify(ToolInvocation{
		Command: "echo ok",
		Files:   []string{"infra/main.tf"},
	})
	if justify.Risk != "justify" {
		t.Fatalf("expected justify risk from policy path, got %q", justify.Risk)
	}
}

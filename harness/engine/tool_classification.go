package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

type ToolClassification struct {
	Risk   string
	Reason string
	Rule   string
}

type ToolInvocation struct {
	Command string
	Args    []string
	Cwd     string
	Files   []string
}

type ToolClassifier struct {
	BlockedCommandPrefixes []string
	JustifyCommandPrefixes []string
	BlockedPathPrefixes    []string
	JustifyPathPrefixes    []string
}

type ClassificationPolicy struct {
	SensitivePaths         []string `json:"sensitive_paths"`
	RiskyPaths             []string `json:"risky_paths"`
	BlockedCommandPrefixes []string `json:"blocked_command_prefixes"`
	JustifyCommandPrefixes []string `json:"justify_command_prefixes"`
}

func NewDefaultToolClassifier() ToolClassifier {
	return ToolClassifier{
		BlockedCommandPrefixes: []string{
			"rm -rf /",
			"mkfs",
			"dd if=",
			"shutdown",
			"reboot",
		},
		JustifyCommandPrefixes: []string{
			"git push --force",
			"git reset --hard",
			"docker system prune",
			"docker volume rm",
		},
		BlockedPathPrefixes: []string{
			"/etc",
			"/root",
			"/home",
			"~/.ssh",
			".ssh",
		},
		JustifyPathPrefixes: []string{
			".github",
			"infra",
			"deploy",
			"docker-compose.yml",
			"Dockerfile",
		},
	}
}

func NewToolClassifierFromPolicy(policy ClassificationPolicy) ToolClassifier {
	base := NewDefaultToolClassifier()
	if len(policy.SensitivePaths) > 0 {
		base.BlockedPathPrefixes = uniqueNormalized(policy.SensitivePaths)
	}
	if len(policy.RiskyPaths) > 0 {
		base.JustifyPathPrefixes = uniqueNormalized(policy.RiskyPaths)
	}
	if len(policy.BlockedCommandPrefixes) > 0 {
		base.BlockedCommandPrefixes = uniqueNormalized(policy.BlockedCommandPrefixes)
	}
	if len(policy.JustifyCommandPrefixes) > 0 {
		base.JustifyCommandPrefixes = uniqueNormalized(policy.JustifyCommandPrefixes)
	}
	return base
}

func LoadClassificationPolicy(policyPath string) (ClassificationPolicy, error) {
	b, err := os.ReadFile(policyPath)
	if err != nil {
		return ClassificationPolicy{}, err
	}
	var policy ClassificationPolicy
	if err := json.Unmarshal(b, &policy); err != nil {
		return ClassificationPolicy{}, err
	}
	return policy, nil
}

func ToolClassifierFromPolicyFile(policyPath string) ToolClassifier {
	policy, err := LoadClassificationPolicy(policyPath)
	if err != nil {
		return NewDefaultToolClassifier()
	}
	return NewToolClassifierFromPolicy(policy)
}

func PolicyPathFromEventsPath(eventsPath string) string {
	return filepath.Join(filepath.Dir(eventsPath), "policy.json")
}

func (c ToolClassifier) Classify(inv ToolInvocation) ToolClassification {
	command := normalizeCommand(inv.Command, inv.Args)
	for _, prefix := range c.BlockedCommandPrefixes {
		if hasPrefix(command, prefix) {
			return ToolClassification{
				Risk:   "block",
				Reason: "command matches blocked pattern",
				Rule:   "blocked_command_prefix:" + prefix,
			}
		}
	}
	for _, prefix := range c.JustifyCommandPrefixes {
		if hasPrefix(command, prefix) {
			return ToolClassification{
				Risk:   "justify",
				Reason: "command matches review-required pattern",
				Rule:   "justify_command_prefix:" + prefix,
			}
		}
	}

	for _, file := range inv.Files {
		normFile := normalizePath(file)
		for _, prefix := range c.BlockedPathPrefixes {
			if hasPrefix(normFile, normalizePath(prefix)) {
				return ToolClassification{
					Risk:   "block",
					Reason: "file path matches blocked prefix",
					Rule:   "blocked_path_prefix:" + prefix,
				}
			}
		}
		for _, prefix := range c.JustifyPathPrefixes {
			if hasPrefix(normFile, normalizePath(prefix)) {
				return ToolClassification{
					Risk:   "justify",
					Reason: "file path matches review-required prefix",
					Rule:   "justify_path_prefix:" + prefix,
				}
			}
		}
	}

	return ToolClassification{
		Risk:   "safe",
		Reason: "no risky command or path pattern matched",
		Rule:   "default_safe",
	}
}

func normalizeCommand(command string, args []string) string {
	cmd := strings.TrimSpace(command)
	if cmd == "" && len(args) > 0 {
		cmd = strings.Join(args, " ")
	}
	return strings.ToLower(strings.TrimSpace(cmd))
}

func normalizePath(path string) string {
	path = strings.TrimSpace(path)
	path = strings.ReplaceAll(path, "\\", "/")
	return strings.ToLower(path)
}

func hasPrefix(value, prefix string) bool {
	value = strings.TrimSpace(strings.ToLower(value))
	prefix = strings.TrimSpace(strings.ToLower(prefix))
	if value == "" || prefix == "" {
		return false
	}
	return strings.HasPrefix(value, prefix)
}

func uniqueNormalized(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, 0, len(in))
	seen := map[string]bool{}
	for _, v := range in {
		norm := strings.TrimSpace(strings.ToLower(v))
		if norm == "" || seen[norm] {
			continue
		}
		seen[norm] = true
		out = append(out, v)
	}
	return out
}

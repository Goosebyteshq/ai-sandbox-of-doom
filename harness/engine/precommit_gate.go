package engine

import (
	"path/filepath"
	"strings"
)

const (
	GatePreCommit = "pre-commit"
)

const (
	GateDecisionPass  = "pass"
	GateDecisionBlock = "block"
)

type PreCommitGateInput struct {
	Agent string

	StagedFiles              []string
	InScopePathPrefixes      []string
	GeneratedFilePatterns    []string
	NonObviousFiles          []string
	NonObviousJustifications map[string]string

	RequireNonObviousJustification bool
	RequireGreenTestsBeforeCommit  bool
	LastFastTestResult             string
	MeaningfulEditsSinceFastTest   bool
}

type GateResult struct {
	Gate     string
	Decision string
	Risk     string
	Reasons  []string
	Payload  map[string]any
}

func EvaluatePreCommitGate(input PreCommitGateInput) GateResult {
	reasons := []string{}
	payload := map[string]any{}

	staged := normalizePaths(input.StagedFiles)
	if len(staged) == 0 {
		reasons = append(reasons, "no staged files")
	}

	generated := DetectGeneratedFiles(staged, input.GeneratedFilePatterns)
	if len(generated) > 0 {
		reasons = append(reasons, "generated files staged")
		payload["generated_files"] = generated
	}

	outOfScope := DetectOutOfScopeFiles(staged, input.InScopePathPrefixes)
	if len(outOfScope) > 0 {
		reasons = append(reasons, "out-of-scope files staged")
		payload["out_of_scope_files"] = outOfScope
	}

	if input.RequireNonObviousJustification {
		missing := MissingJustifications(input.NonObviousFiles, input.NonObviousJustifications)
		if len(missing) > 0 {
			reasons = append(reasons, "missing non-obvious file justifications")
			payload["missing_justifications"] = missing
		}
	}

	if input.RequireGreenTestsBeforeCommit && input.MeaningfulEditsSinceFastTest {
		last := strings.ToLower(strings.TrimSpace(input.LastFastTestResult))
		if last != "pass" {
			reasons = append(reasons, "fast tests are stale or failing since meaningful edits")
			payload["last_fast_test_result"] = input.LastFastTestResult
		}
	}

	decision := GateDecisionPass
	risk := "safe"
	if len(reasons) > 0 {
		decision = GateDecisionBlock
		risk = "block"
	}

	payload["checks_run"] = []string{
		"staged_files_present",
		"generated_files",
		"scope",
		"justifications",
		"fast_tests_freshness",
	}
	payload["reason_count"] = len(reasons)

	return GateResult{
		Gate:     GatePreCommit,
		Decision: decision,
		Risk:     risk,
		Reasons:  reasons,
		Payload:  payload,
	}
}

func (b *Bus) EmitPreCommitGateDecision(agent string, result GateResult) error {
	return b.EmitGateDecision(
		agent,
		result.Gate,
		result.Decision,
		buildGateMessage(result),
		result.Risk,
		result.Payload,
	)
}

func DetectGeneratedFiles(files []string, patterns []string) []string {
	if len(files) == 0 || len(patterns) == 0 {
		return nil
	}
	matches := []string{}
	for _, file := range files {
		if matchesAnyPattern(file, patterns) {
			matches = append(matches, file)
		}
	}
	return matches
}

func DetectOutOfScopeFiles(files []string, inScopePrefixes []string) []string {
	files = normalizePaths(files)
	inScopePrefixes = normalizePaths(inScopePrefixes)
	if len(files) == 0 || len(inScopePrefixes) == 0 {
		return nil
	}
	out := []string{}
	for _, file := range files {
		if !matchesAnyPrefix(file, inScopePrefixes) {
			out = append(out, file)
		}
	}
	return out
}

func MissingJustifications(files []string, justifications map[string]string) []string {
	files = normalizePaths(files)
	if len(files) == 0 {
		return nil
	}
	missing := []string{}
	for _, file := range files {
		why := strings.TrimSpace(justifications[file])
		if why == "" {
			missing = append(missing, file)
		}
	}
	return missing
}

func buildGateMessage(result GateResult) string {
	if result.Decision == GateDecisionPass {
		return "Pre-commit gate passed"
	}
	return "Pre-commit gate blocked"
}

func matchesAnyPattern(path string, patterns []string) bool {
	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		if strings.HasSuffix(pattern, "/") {
			if strings.HasPrefix(path, normalizePath(pattern)) {
				return true
			}
			continue
		}
		if pattern == path {
			return true
		}
		ok, err := filepath.Match(pattern, filepath.Base(path))
		if err == nil && ok {
			return true
		}
		ok, err = filepath.Match(pattern, path)
		if err == nil && ok {
			return true
		}
	}
	return false
}

func matchesAnyPrefix(path string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

func normalizePaths(paths []string) []string {
	if len(paths) == 0 {
		return nil
	}
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		v := normalizePath(path)
		if v == "" {
			continue
		}
		out = append(out, v)
	}
	return out
}

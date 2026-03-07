package harness

import (
	"bufio"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type config struct {
	Version                   int    `json:"version"`
	Provider                  string `json:"provider"`
	AdversarialIntervalMinute int    `json:"adversarial_interval_minutes"`
}

type todoFile struct {
	Version int        `json:"version"`
	Items   []todoItem `json:"items"`
}

type todoItem struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

type event struct {
	Timestamp string `json:"timestamp"`
	Type      string `json:"type"`
	Agent     string `json:"agent"`
	Message   string `json:"message"`
}

//go:embed scaffold/**
var scaffoldFS embed.FS

// RunWithSession wraps an active agent run with harness lifecycle behavior.
// It is Codex-enabled today and no-op for other agents.
func RunWithSession(agent, projectPath string, out io.Writer, runFn func() error) error {
	end, err := startSession(agent, projectPath, out)
	if err != nil {
		return err
	}
	runErr := runFn()
	end(runErr)
	return runErr
}

// WriteScaffold writes harness runtime assets into a destination directory.
func WriteScaffold(dstRoot string) error {
	if err := os.MkdirAll(dstRoot, 0755); err != nil {
		return err
	}

	return fs.WalkDir(scaffoldFS, "scaffold", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(path, "scaffold")
		rel = strings.TrimPrefix(rel, "/")
		if rel == "" {
			return nil
		}
		target := filepath.Join(dstRoot, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		data, readErr := scaffoldFS.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		mode := fs.FileMode(0644)
		if strings.HasSuffix(target, ".sh") {
			mode = 0755
		}
		return os.WriteFile(target, data, mode)
	})
}

func startSession(agent, projectPath string, out io.Writer) (func(error), error) {
	if agent != "codex" {
		return func(error) {}, nil
	}

	now := time.Now().UTC()
	cfg, err := ensureHarnessFiles(projectPath, agent, now)
	if err != nil {
		return nil, err
	}
	if err := appendEvent(projectPath, event{
		Timestamp: now.Format(time.RFC3339),
		Type:      "session_start",
		Agent:     agent,
		Message:   "Codex session started",
	}); err != nil {
		return nil, err
	}
	_ = ensureAdversarialTodo(projectPath, agent, cfg, now, "session_start")

	fmt.Fprintf(out, "Harness: codex logging enabled at %s/.doombox\n", projectPath)
	fmt.Fprintln(out, "Harness: important decisions should be appended to session-log.jsonl.")

	stop := make(chan struct{})
	done := make(chan struct{})
	var once sync.Once

	go func() {
		defer close(done)
		interval := time.Duration(cfg.AdversarialIntervalMinute) * time.Minute
		if interval <= 0 {
			return
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case tick := <-ticker.C:
				_ = ensureAdversarialTodo(projectPath, agent, cfg, tick.UTC(), "interval_tick")
			}
		}
	}()

	end := func(runErr error) {
		once.Do(func() {
			close(stop)
			<-done
			msg := "Codex session ended"
			if runErr != nil {
				msg = "Codex session ended with error: " + runErr.Error()
			}
			_ = appendEvent(projectPath, event{
				Timestamp: time.Now().UTC().Format(time.RFC3339),
				Type:      "session_end",
				Agent:     agent,
				Message:   msg,
			})
		})
	}
	return end, nil
}

func ensureHarnessFiles(projectPath, agent string, now time.Time) (config, error) {
	dir := filepath.Join(projectPath, ".doombox")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return config{}, err
	}

	cfgPath := filepath.Join(dir, "harness.json")
	cfg := config{
		Version:                   1,
		Provider:                  agent,
		AdversarialIntervalMinute: 10,
	}
	cfgExists := false
	if b, err := os.ReadFile(cfgPath); err == nil {
		cfgExists = true
		var parsed config
		if json.Unmarshal(b, &parsed) == nil && parsed.Version > 0 {
			cfg = parsed
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return config{}, err
	}
	if cfg.Provider == "" {
		cfg.Provider = agent
	}
	if cfg.AdversarialIntervalMinute <= 0 {
		cfg.AdversarialIntervalMinute = 10
	}
	if !cfgExists {
		if err := writeJSONFile(cfgPath, cfg); err != nil {
			return config{}, err
		}
	}

	todoPath := filepath.Join(dir, "todo.json")
	if _, err := os.Stat(todoPath); errors.Is(err, os.ErrNotExist) {
		if err := writeJSONFile(todoPath, todoFile{Version: 1, Items: []todoItem{}}); err != nil {
			return config{}, err
		}
	} else if err != nil {
		return config{}, err
	}

	logPath := filepath.Join(dir, "session-log.jsonl")
	if _, err := os.Stat(logPath); errors.Is(err, os.ErrNotExist) {
		if err := os.WriteFile(logPath, []byte{}, 0644); err != nil {
			return config{}, err
		}
	} else if err != nil {
		return config{}, err
	}

	_ = appendEvent(projectPath, event{
		Timestamp: now.Format(time.RFC3339),
		Type:      "harness_ready",
		Agent:     agent,
		Message:   "Harness files initialized",
	})

	return cfg, nil
}

func ensureAdversarialTodo(projectPath, agent string, cfg config, now time.Time, reason string) error {
	last, err := lastEventTimestamp(projectPath, "adversarial_check_due")
	if err != nil {
		return err
	}
	interval := time.Duration(cfg.AdversarialIntervalMinute) * time.Minute
	if !last.IsZero() && now.Sub(last) < interval {
		return nil
	}

	todoPath := filepath.Join(projectPath, ".doombox", "todo.json")
	todos, err := readTodoFile(todoPath)
	if err != nil {
		return err
	}
	for _, item := range todos.Items {
		if item.Type == "adversarial_check" && item.Status == "open" {
			return nil
		}
	}

	id := "adv-" + strconv.FormatInt(now.Unix(), 10)
	todos.Items = append(todos.Items, todoItem{
		ID:        id,
		Type:      "adversarial_check",
		Title:     "Run adversarial drift check against last 10 minutes of work",
		Status:    "open",
		CreatedAt: now.Format(time.RFC3339),
	})
	if err := writeJSONFile(todoPath, todos); err != nil {
		return err
	}

	return appendEvent(projectPath, event{
		Timestamp: now.Format(time.RFC3339),
		Type:      "adversarial_check_due",
		Agent:     agent,
		Message:   "Queued adversarial check (" + reason + ")",
	})
}

func appendEvent(projectPath string, e event) error {
	p := filepath.Join(projectPath, ".doombox", "session-log.jsonl")
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(append(b, '\n')); err != nil {
		return err
	}
	return nil
}

func lastEventTimestamp(projectPath, eventType string) (time.Time, error) {
	p := filepath.Join(projectPath, ".doombox", "session-log.jsonl")
	b, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return time.Time{}, nil
		}
		return time.Time{}, err
	}
	var latest time.Time
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var e event
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			continue
		}
		if e.Type != eventType {
			continue
		}
		ts, err := time.Parse(time.RFC3339, e.Timestamp)
		if err != nil {
			continue
		}
		if ts.After(latest) {
			latest = ts
		}
	}
	return latest, nil
}

func readTodoFile(path string) (todoFile, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return todoFile{}, err
	}
	var todos todoFile
	if err := json.Unmarshal(b, &todos); err != nil {
		return todoFile{}, err
	}
	if todos.Version == 0 {
		todos.Version = 1
	}
	if todos.Items == nil {
		todos.Items = []todoItem{}
	}
	return todos, nil
}

func writeJSONFile(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(path, b, 0644)
}

func confirmYes(in io.Reader, out io.Writer, prompt string) (bool, error) {
	fmt.Fprint(out, prompt)
	reader := bufio.NewReader(in)
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	return strings.EqualFold(strings.TrimSpace(line), "yes"), nil
}

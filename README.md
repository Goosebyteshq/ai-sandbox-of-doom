# AI Agent Docker Sandbox

Run Claude, Codex, or Gemini in YOLO/unsafe mode inside Docker, with only one project directory mounted.

## Install the typed CLI

```bash
go install github.com/Goosebyteshq/doombox/cmd/doombox@latest
```

Then use:

```bash
doombox open --agent codex /path/to/project
doombox list
doombox harness init /path/to/project
doombox harness status /path/to/project
doombox harness status --json /path/to/project
doombox harness score /path/to/project
doombox harness score --json /path/to/project
doombox harness report /path/to/project
doombox harness report --json /path/to/project
doombox harness report --strict --min-score 0.8 /path/to/project
doombox harness export-eval --out eval/current.json /path/to/project
doombox harness compare /path/to/baseline /path/to/candidate
doombox harness flip --baseline baseline.json --candidate candidate.json
doombox harness flip --strict --max-regressions 0 --baseline baseline.json --candidate candidate.json
doombox harness help
```

`harness flip` accepts:
- `[]EvalRun`
- `{"runs":[...]}`
- `harness report --json` output (single object or array)
- `harness export-eval` output (`EvalRun` JSON object)

If you run `doombox open` without a path, it will use your current directory and ask for explicit confirmation before mounting it.

This project expects global CLI install only (no `go run` workflow).

## Local multi-module development (safe)

If you want local multi-module wiring (for example, splitting `harness/` later), use a local Go workspace instead of `replace` in root `go.mod`.

```bash
go work init .
go work use ./harness
# optional explicit mapping:
go work edit -replace github.com/Goosebyteshq/doombox/harness=./harness
```

Rules:
- Keep root `go.mod` free of local `replace ... => ./...` directives.
- Do not commit `go.work` / `go.work.sum` (they are local-only).
- `make fast-check` enforces this with `scripts/check-no-local-replace.sh`.

## Why this setup

- Agents can run with full autonomy in-container.
- Only the project path you pass is bind-mounted to `/workspace/project`.
- Each project gets its own container runtime and its own home volume.
- Go and Playwright workflows run inside the container.
- Codex sessions can keep structured JSON context under `.doombox/`.

## What is installed

- Claude Code CLI
- OpenAI Codex CLI
- Gemini CLI
- GitHub CLI (`gh`)
- Fast search/navigation: `ripgrep`, `fd`, `bat`, `fzf`
- Data helpers: `jq`, `yq`
- Shell/productivity: `tmux`, `tree`, `entr`, `direnv`, `shellcheck`
- Go 1.23.5
- Node.js + pnpm
- Python3 + uv
- Playwright Chromium + system deps

## Quick start

Open a project with Claude:

```bash
doombox open --agent claude /path/to/project
```

Open a specific project with Codex:

```bash
doombox open --agent codex /path/to/project
```

Open (connect if running, otherwise start):

```bash
doombox open --agent codex /path/to/project
```

Start with Gemini in detached mode:

```bash
doombox open --agent gemini -d /path/to/project
```

`doombox open` is the primary and only project session command.

## Codex Harness (JSON)

When you run `doombox open --agent codex ...`, doombox initializes:

- `.doombox/harness.json` (provider + interval config)
- `.doombox/todo.json` (structured task list)
- `.doombox/session-log.jsonl` (append-only event log)
- `.doombox/policy.json` (checkpoint/test/gate + tool-risk policy)
- `.doombox/events.jsonl` (typed event bus for supervisor/gates)
- `.doombox/checkpoints/*.json` (structured checkpoint snapshots)
- `.doombox/permission-denials.jsonl` (permission-related blocks/approval events)

Session behavior for Codex:

- Writes `session_start` / `session_end` events to both logs.
- Emits typed `checkpoint_due` events when adversarial checks are queued.
- Persists a checkpoint snapshot and emits `checkpoint_written`.
- Auto-queues an `adversarial_check` TODO when due.
- Runs a periodic timer (default 10 minutes) while the Codex session is active.

Typed bus writer support is now in `harness/engine/bus.go` with helpers for:

- tool invocations (`tool_invocation`)
- edit clusters (`edit_cluster`)
- test results (`test_result`)
- gate decisions (`gate_decision`)

Tool invocation classification support is in `harness/engine/tool_classification.go`:

- `safe`
- `justify`
- `block`
- rule sources: `sensitive_paths`, `risky_paths`, `blocked_command_prefixes`, `justify_command_prefixes` in `.doombox/policy.json`

Action-based checkpoint triggering is implemented in `harness/engine/checkpoint_trigger.go`:

- emits `checkpoint_due` every configurable N action clusters (default 4)

Immediate checkpoint triggering is implemented in `harness/engine/immediate_trigger.go`:

- failing tests
- risky path touches
- large diffs
- pre-commit/pre-push signals

Pre-commit gate evaluator is in `harness/engine/precommit_gate.go`:

- blocks generated files
- blocks out-of-scope files
- requires non-obvious file justifications when enabled
- blocks commit when fast tests are stale/failing after meaningful edits

Test discipline helpers are in `harness/engine/test_discipline.go`:

- run fast test commands after meaningful edits
- run integration test commands on policy-triggered checkpoints

Pre-push gate evaluator is in `harness/engine/prepush_gate.go`:

- blocks push when integration tests are stale/failing under pre-push policy

Trajectory rubric scoring is in `harness/engine/rubric.go`:

- computes scope, test, safety, and efficiency subscores
- returns a composite score for harness/eval comparisons

Canary rollout helper is in `harness/engine/canary.go`:

- deterministic policy canary assignment by run id and rollout percent

Flip analysis utility is in `harness/engine/flip_analysis.go`:

- compares baseline vs candidate runs
- highlights pass->fail regressions and fail->pass improvements

Current defaults are Codex-specific, but the config shape is provider-based so Gemini/Cloud can be added to the same harness flow later.

Provider adapter registry is in `harness/adapters/provider.go`:

- `codex`: primary adapter
- `gemini`: stub adapter
- `cloud` (claude alias): stub adapter

Interactive `doombox open` launches a tmux session inside the container:

- pane 1: selected agent CLI
- pane 2: harness supervisor HUD (events/checkpoints/todos/risk)
- pane 3: live `.doombox/events.jsonl` tail

## Harness testing (no live LLM required)

You can validate harness behavior without calling an LLM:

- Unit tests in `harness/session_test.go` verify lifecycle, `.doombox` initialization, and adversarial TODO scheduling.
- `make test` runs both root CLI tests and harness-module tests.
- `make test-cli-smoke` validates the CLI help/command wiring without Docker.
- `make fast-check` runs formatting, vetting, tests, and compose validation.
- `make test-harness-sim` runs deterministic harness simulator tests only.

Current mock harness assets:

- Adapter: `harness/adapters/mock`
- Example fixture: `harness/fixtures/mock/basic-flow.json`
- Checkpoint/gate fixture: `harness/fixtures/mock/checkpoint-gate-flow.json`
- Fixture-driven integration tests: `harness/adapters/mock/mock_integration_test.go`
- Replay tests from generated `events.jsonl`: `harness/adapters/mock/replay_test.go`

CI also runs the simulator suite in a dedicated `harness-simulator` job.

## Agent shortcuts

```bash
./start-claude.sh /path/to/project
./start-codex.sh /path/to/project
./start-gemini.sh /path/to/project
```

Shortcuts call the globally installed `doombox` binary.

## Safety model

Container mounts:

- `${PROJECT_PATH}:/workspace/project` (only host project mount)
- `${AI_HOME_VOLUME}:/home/developer` (project-specific Docker volume)
- `/var/run/docker.sock` (for Docker CLI use inside container)

## Useful commands

Rebuild image:

```bash
docker-compose build --no-cache
```

Run the full integration suite:

```bash
make test-integration
```

Run fast local checks (format/lint/unit/config):

```bash
make fast-check
```

Keep the integration container running after tests:

```bash
KEEP_INTEGRATION_CONTAINER=1 make test-integration
```

See running containers:

```bash
doombox list
doombox list --all
```

Open shell in a container:

```bash
docker exec -it ai-dev-<project-name> bash
```

## Hooks and CI

Install Lefthook hooks:

```bash
lefthook install
```

- `pre-commit`: `gofmt`, fast checks, unit tests
- GitHub Actions CI runs both fast checks and integration on every PR/push

## Inside container examples

```bash
go test ./...
npx playwright test
pnpm test
```

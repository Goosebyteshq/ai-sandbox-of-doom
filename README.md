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
```

If you run `doombox open` without a path, it will use your current directory and ask for explicit confirmation before mounting it.

This project expects global CLI install only (no `go run` workflow).

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

`doombox open` is the primary command. `start` and `connect` route to the same flow.

## Codex Harness (JSON)

When you run `doombox open --agent codex ...`, doombox initializes:

- `.doombox/harness.json` (provider + interval config)
- `.doombox/todo.json` (structured task list)
- `.doombox/session-log.jsonl` (append-only event log)

Session behavior for Codex:

- Writes `session_start` / `session_end` events.
- Auto-queues an `adversarial_check` TODO when due.
- Runs a periodic timer (default 10 minutes) while the Codex session is active.

Current defaults are Codex-specific, but the config shape is provider-based so Gemini/Cloud can be added to the same harness flow later.

## Harness testing (no live LLM required)

You can validate harness behavior without calling an LLM:

- Unit tests in `harness/session_test.go` verify lifecycle, `.doombox` initialization, and adversarial TODO scheduling.
- `go test ./...` runs both CLI and harness package tests.
- `make fast-check` runs formatting, vetting, tests, and compose validation.

Planned next step:

- Add a deterministic mock-agent adapter and fixture-driven harness integration tests so supervisor triggers and commit/push gates can be validated in CI with zero model calls.

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

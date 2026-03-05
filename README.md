# AI Agent Docker Sandbox

Run Claude, Codex, or Gemini in YOLO/unsafe mode inside Docker, with only one project directory mounted.

## Install the typed CLI

```bash
go install github.com/Goosebyteshq/doombox/cmd/doombox@latest
```

Then use:

```bash
doombox start --agent claude
doombox open --agent codex /path/to/project
doombox connect --agent codex <project-name>
```

This project expects global CLI install only (no `go run` workflow).

## Why this setup

- Agents can run with full autonomy in-container.
- Only the project path you pass is bind-mounted to `/workspace/project`.
- Each project gets its own container runtime and its own home volume.
- Go and Playwright workflows run inside the container.

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

Start in current directory with Claude:

```bash
doombox start /path/to/project
```

Start a specific project with Codex:

```bash
doombox start --agent codex /path/to/project
```

Open (connect if running, otherwise start):

```bash
doombox open --agent codex /path/to/project
```

Start with Gemini in detached mode:

```bash
doombox start --agent gemini -d /path/to/project
```

Reconnect to an existing container:

```bash
doombox connect --agent codex project-name
```

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
docker ps --filter "name=ai-dev-"
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

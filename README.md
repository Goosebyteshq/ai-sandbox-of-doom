# AI Agent Docker Sandbox

Run Claude, Codex, or Gemini in YOLO/unsafe mode inside Docker, with only one project directory mounted.

## Why this setup

- Agents can run with full autonomy in-container.
- Only the project path you pass is bind-mounted to `/workspace/project`.
- Agent home/config is stored in a Docker volume, not your host home.
- Go and Playwright workflows run inside the container.

## What is installed

- Claude Code CLI
- OpenAI Codex CLI
- Gemini CLI
- Go 1.23.5
- Node.js + pnpm
- Python3 + uv
- Playwright Chromium + system deps

## Quick start

Start in current directory with Claude:

```bash
./start-agent.sh
```

Start a specific project with Codex:

```bash
./start-agent.sh --agent codex /path/to/project
```

Start with Gemini in detached mode:

```bash
./start-agent.sh --agent gemini -d /path/to/project
```

Reconnect to an existing container:

```bash
./connect.sh --agent codex project-name
```

## Agent shortcuts

```bash
./start-claude.sh /path/to/project
./start-codex.sh /path/to/project
./start-gemini.sh /path/to/project
```

## Safety model

Container mounts:

- `${PROJECT_PATH}:/workspace/project` (only host project mount)
- `ai-dev-home:/home/developer` (Docker volume)
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

## Inside container examples

```bash
go test ./...
npx playwright test
pnpm test
```

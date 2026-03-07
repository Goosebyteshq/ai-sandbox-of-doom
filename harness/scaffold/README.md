# Doombox Harness Runtime

This directory is injected into the development container at build time.

- `policy.json` holds runtime guardrail and checkpoint defaults.
- `scripts/` contains supervisor and hook helpers.
- `scripts/launch_tmux.sh` starts tmux with `windows` layout by default:
- `agent`, `supervisor`, `events`, and `shell` windows.
- Supports `compact` layout for a single-window pane view.
- `scripts/supervisor.sh` renders a live HUD from `.doombox/events.jsonl`.

This scaffold is designed to be extraction-friendly for a future standalone harness repository.

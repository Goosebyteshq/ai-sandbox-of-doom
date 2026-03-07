# Doombox Harness Runtime

This directory is injected into the development container at build time.

- `policy.json` holds runtime guardrail and checkpoint defaults.
- `scripts/` is reserved for supervisor and hook helpers.
- `scripts/launch_tmux.sh` starts agent, supervisor, and event panes.

This scaffold is designed to be extraction-friendly for a future standalone harness repository.

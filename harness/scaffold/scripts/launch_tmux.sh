#!/bin/bash
set -euo pipefail

agent_cmd="${DOOMBOX_AGENT_CMD:-}"
session_name="${DOOMBOX_TMUX_SESSION:-doombox}"
supervisor_cmd="${DOOMBOX_SUPERVISOR_CMD:-/opt/doombox/harness/scripts/supervisor.sh}"
events_cmd="${DOOMBOX_EVENTS_CMD:-mkdir -p .doombox && touch .doombox/events.jsonl && tail -f .doombox/events.jsonl}"

if [[ -z "${agent_cmd}" ]]; then
  echo "missing DOOMBOX_AGENT_CMD for tmux launch"
  exit 1
fi

if [[ "${DOOMBOX_DISABLE_TMUX:-0}" == "1" ]] || ! command -v tmux >/dev/null 2>&1; then
  exec bash -lc "${agent_cmd}"
fi

if tmux has-session -t "${session_name}" 2>/dev/null; then
  exec tmux attach -t "${session_name}"
fi

tmux new-session -d -s "${session_name}" "bash -lc '${agent_cmd}'"
tmux split-window -h -t "${session_name}:0" "bash -lc '${supervisor_cmd}'"
tmux split-window -v -t "${session_name}:0.1" "bash -lc '${events_cmd}'"
tmux select-pane -t "${session_name}:0.0"

exec tmux attach -t "${session_name}"

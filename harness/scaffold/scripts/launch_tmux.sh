#!/bin/bash
set -euo pipefail

agent_cmd="${DOOMBOX_AGENT_CMD:-}"
session_name="${DOOMBOX_TMUX_SESSION:-doombox}"
supervisor_cmd="${DOOMBOX_SUPERVISOR_CMD:-/opt/doombox/harness/scripts/supervisor.sh}"
events_cmd="${DOOMBOX_EVENTS_CMD:-mkdir -p .doombox && touch .doombox/events.jsonl && tail -f .doombox/events.jsonl}"
layout="${DOOMBOX_LAYOUT:-windows}"

if [[ -z "${agent_cmd}" ]]; then
  echo "missing DOOMBOX_AGENT_CMD for tmux launch"
  exit 1
fi

if [[ "${DOOMBOX_DISABLE_TMUX:-0}" == "1" ]] || ! command -v tmux >/dev/null 2>&1; then
  exec bash -lc "${agent_cmd}"
fi

if tmux has-session -t "${session_name}" 2>/dev/null; then
  echo "Doombox tmux session: ${session_name}"
  echo "Tmux quick help: Ctrl-b + n/p (next/prev), Ctrl-b + 1..4 (window), Ctrl-b + d (detach)"
  exec tmux attach -t "${session_name}"
fi

case "${layout}" in
  windows)
    tmux new-session -d -s "${session_name}" -n agent "bash -lc '${agent_cmd}'"
    tmux new-window -t "${session_name}" -n supervisor "bash -lc '${supervisor_cmd}'"
    tmux new-window -t "${session_name}" -n events "bash -lc '${events_cmd}'"
    tmux new-window -t "${session_name}" -n shell "bash -l"
    tmux select-window -t "${session_name}:agent"
    ;;
  compact)
    tmux new-session -d -s "${session_name}" -n compact "bash -lc '${agent_cmd}'"
    tmux split-window -h -t "${session_name}:compact" "bash -lc '${supervisor_cmd}'"
    tmux split-window -v -t "${session_name}:compact.1" "bash -lc '${events_cmd}'"
    tmux split-window -v -t "${session_name}:compact.0" "bash -l"
    tmux select-layout -t "${session_name}:compact" tiled
    tmux select-pane -t "${session_name}:compact.0"
    ;;
  *)
    echo "unsupported DOOMBOX_LAYOUT: ${layout} (expected windows|compact)"
    exit 1
    ;;
esac

echo "Doombox tmux session: ${session_name} (${layout})"
echo "Tmux quick help: Ctrl-b + n/p (next/prev), Ctrl-b + 1..4 (window), Ctrl-b + d (detach)"
if [[ "${layout}" == "windows" ]]; then
  echo "Windows: 1=agent 2=supervisor 3=events 4=shell"
fi

exec tmux attach -t "${session_name}"

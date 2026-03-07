#!/bin/bash
set -euo pipefail

agent_cmd="${DOOMBOX_AGENT_CMD:-}"
session_name="${DOOMBOX_TMUX_SESSION:-doombox}"
supervisor_cmd="${DOOMBOX_SUPERVISOR_CMD:-/opt/doombox/harness/scripts/supervisor.sh}"
events_cmd="${DOOMBOX_EVENTS_CMD:-mkdir -p .doombox && touch .doombox/events.jsonl && tail -f .doombox/events.jsonl}"
layout="${DOOMBOX_LAYOUT:-windows}"
shell_cmd="${DOOMBOX_SHELL_CMD:-}"

if [[ -z "${shell_cmd}" ]]; then
  if command -v zsh >/dev/null 2>&1; then
    shell_cmd="zsh -l"
  else
    shell_cmd="bash -l"
  fi
fi

if [[ -z "${agent_cmd}" ]]; then
  echo "missing DOOMBOX_AGENT_CMD for tmux launch"
  exit 1
fi

if [[ "${DOOMBOX_DISABLE_TMUX:-0}" == "1" ]] || ! command -v tmux >/dev/null 2>&1; then
  exec bash -lc "${agent_cmd}"
fi

if tmux has-session -t "${session_name}" 2>/dev/null; then
  echo "Doombox tmux session: ${session_name}"
  echo "Tmux quick help: Ctrl-g (detach), Ctrl-b + n/p (next/prev), Ctrl-b + 1..4 (window), Ctrl-b + d (detach)"
  exec tmux attach -t "${session_name}"
fi

case "${layout}" in
  windows)
    tmux new-session -d -s "${session_name}" -n agent "bash -lc '${agent_cmd}'"
    tmux new-window -t "${session_name}" -n supervisor "bash -lc '${supervisor_cmd}'"
    tmux new-window -t "${session_name}" -n events "bash -lc '${events_cmd}'"
    tmux new-window -t "${session_name}" -n shell "bash -lc '${shell_cmd}'"
    tmux select-window -t "${session_name}:agent"
    ;;
  compact)
    tmux new-session -d -s "${session_name}" -n compact "bash -lc '${agent_cmd}'"
    tmux split-window -h -t "${session_name}:compact" "bash -lc '${supervisor_cmd}'"
    tmux split-window -v -t "${session_name}:compact.1" "bash -lc '${events_cmd}'"
    tmux split-window -v -t "${session_name}:compact.0" "bash -lc '${shell_cmd}'"
    tmux select-layout -t "${session_name}:compact" tiled
    tmux select-pane -t "${session_name}:compact.0"
    ;;
  *)
    echo "unsupported DOOMBOX_LAYOUT: ${layout} (expected windows|compact)"
    exit 1
    ;;
esac

# Make exiting obvious for non-tmux users:
# - Ctrl-g detaches immediately without a prefix
# - prefix + q also detaches
tmux bind-key -T root C-g detach-client
tmux bind-key -T prefix q detach-client
tmux set-option -t "${session_name}" -g status-right "C-g detach | C-b q detach | C-b ? help"

echo "Doombox tmux session: ${session_name} (${layout})"
echo "Tmux quick help: Ctrl-g (detach), Ctrl-b + n/p (next/prev), Ctrl-b + 1..4 (window), Ctrl-b + d (detach)"
if [[ "${layout}" == "windows" ]]; then
  echo "Windows: 1=agent 2=supervisor 3=events 4=shell"
fi

exec tmux attach -t "${session_name}"

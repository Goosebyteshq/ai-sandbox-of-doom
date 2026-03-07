#!/bin/bash
set -euo pipefail

project_path="${PROJECT_PATH:-/workspace/project}"
doombox_dir="${project_path}/.doombox"
events_path="${doombox_dir}/events.jsonl"
checkpoints_dir="${doombox_dir}/checkpoints"
todo_path="${doombox_dir}/todo.json"

mkdir -p "${doombox_dir}" "${checkpoints_dir}"
touch "${events_path}"

if [ ! -f "${todo_path}" ]; then
  cat >"${todo_path}" <<'JSON'
{
  "version": 1,
  "items": []
}
JSON
fi

count_files() {
  local dir="$1"
  local pattern="$2"
  if [ ! -d "${dir}" ]; then
    echo 0
    return
  fi
  find "${dir}" -maxdepth 1 -type f -name "${pattern}" 2>/dev/null | wc -l | tr -d ' '
}

safe_line_count() {
  local path="$1"
  if [ ! -f "${path}" ]; then
    echo 0
    return
  fi
  wc -l <"${path}" | tr -d ' '
}

print_summary() {
  local now
  now="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
  local event_count checkpoint_count
  event_count="$(safe_line_count "${events_path}")"
  checkpoint_count="$(count_files "${checkpoints_dir}" "*.json")"

  local open_todos
  open_todos=0
  if command -v jq >/dev/null 2>&1; then
    open_todos="$(jq -r '[.items[]? | select(.status == "open")] | length' "${todo_path}" 2>/dev/null || echo 0)"
  fi

  local last_event_type last_risk_type
  last_event_type="-"
  last_risk_type="-"
  if command -v jq >/dev/null 2>&1 && [ "${event_count}" -gt 0 ]; then
    last_event_type="$(tail -n 1 "${events_path}" | jq -r '.event_type // "-"' 2>/dev/null || echo "-")"
    last_risk_type="$(tail -n 1 "${events_path}" | jq -r '.risk_classification // "-"' 2>/dev/null || echo "-")"
  fi

  local block_count justify_count
  block_count=0
  justify_count=0
  if command -v jq >/dev/null 2>&1 && [ "${event_count}" -gt 0 ]; then
    block_count="$(jq -r 'select(.risk_classification=="block") | 1' "${events_path}" 2>/dev/null | wc -l | tr -d ' ')"
    justify_count="$(jq -r 'select(.risk_classification=="justify") | 1' "${events_path}" 2>/dev/null | wc -l | tr -d ' ')"
  fi

  clear
  echo "Doombox Supervisor HUD"
  echo "======================"
  echo "time (utc): ${now}"
  echo "project: ${project_path}"
  echo
  echo "events: ${event_count}"
  echo "checkpoints: ${checkpoint_count}"
  echo "open_todos: ${open_todos}"
  echo "risk_block_events: ${block_count}"
  echo "risk_justify_events: ${justify_count}"
  echo "last_event_type: ${last_event_type}"
  echo "last_risk: ${last_risk_type}"
  echo
  echo "last 5 events:"
  if [ "${event_count}" -eq 0 ]; then
    echo "(none)"
    return
  fi
  if command -v jq >/dev/null 2>&1; then
    tail -n 5 "${events_path}" | jq -r '[.timestamp // "-", .event_type // "-", .risk_classification // "-", .message // "-"] | @tsv' 2>/dev/null || tail -n 5 "${events_path}"
  else
    tail -n 5 "${events_path}"
  fi
}

print_summary

tail -n 0 -F "${events_path}" | while IFS= read -r _line; do
  print_summary
done

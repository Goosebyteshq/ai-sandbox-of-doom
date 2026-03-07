#!/bin/bash

set -euo pipefail

MAX_LINES="${GO_MAX_LINES:-1000}"

if ! command -v git >/dev/null 2>&1; then
  echo "git is required to run go max-lines check"
  exit 1
fi

violations=0

while IFS= read -r file; do
  [[ -z "${file}" ]] && continue
  if [[ ! -f "${file}" ]]; then
    continue
  fi
  line_count="$(wc -l <"${file}" | tr -d ' ')"
  if (( line_count > MAX_LINES )); then
    echo "Refactor required: ${file} has ${line_count} lines (max ${MAX_LINES})."
    violations=1
  fi
done < <(git ls-files '*.go')

if (( violations > 0 )); then
  echo ""
  echo "One or more Go files exceed ${MAX_LINES} lines."
  echo "Please refactor these files below ${MAX_LINES} lines and retry."
  exit 1
fi

echo "Go max-lines check passed (max ${MAX_LINES})."

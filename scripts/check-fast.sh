#!/bin/bash

set -euo pipefail

compose_cmd() {
  if command -v docker-compose >/dev/null 2>&1; then
    echo "docker-compose"
    return
  fi
  if command -v docker >/dev/null 2>&1 && docker compose version >/dev/null 2>&1; then
    echo "docker compose"
    return
  fi
  echo ""
}

echo "==> gofmt"
UNFORMATTED="$(gofmt -l .)"
if [[ -n "${UNFORMATTED}" ]]; then
  echo "Go files not formatted:"
  echo "${UNFORMATTED}"
  exit 1
fi

echo "==> go vet"
go vet ./...

echo "==> go test"
go test ./...

echo "==> bash -n"
bash -n start-agent.sh connect.sh start-claude.sh start-codex.sh start-gemini.sh scripts/*.sh

echo "==> shellcheck"
if command -v shellcheck >/dev/null 2>&1; then
  shellcheck start-agent.sh connect.sh start-claude.sh start-codex.sh start-gemini.sh scripts/*.sh
else
  echo "shellcheck not found; skipping local shellcheck"
fi

echo "==> docker compose config"
COMPOSE="$(compose_cmd)"
if [[ -z "${COMPOSE}" ]]; then
  echo "docker compose not found"
  exit 1
fi
${COMPOSE} config >/dev/null

echo "Fast checks passed."

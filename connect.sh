#!/bin/bash

set -euo pipefail

show_help() {
  echo "Usage: $0 [--agent claude|codex|gemini] [PROJECT_NAME]"
}

agent_command() {
  local agent="$1"
  case "$agent" in
    claude)
      echo "claude --dangerously-skip-permissions"
      ;;
    codex)
      echo "codex --sandbox danger-full-access --ask-for-approval never"
      ;;
    gemini)
      echo "gemini --yolo"
      ;;
    *)
      return 1
      ;;
  esac
}

AGENT="${AGENT:-claude}"
POSITIONAL_ARGS=()

while [[ $# -gt 0 ]]; do
  case "$1" in
    -a|--agent)
      AGENT="$2"
      shift 2
      ;;
    -h|--help)
      show_help
      exit 0
      ;;
    -*)
      echo "Unknown option: $1"
      show_help
      exit 1
      ;;
    *)
      POSITIONAL_ARGS+=("$1")
      shift
      ;;
  esac
done

set -- "${POSITIONAL_ARGS[@]}"

PROJECT_NAME="${1:-$(basename "$(pwd)")}"
COMPOSE_PROJECT_NAME="ai-dev-${PROJECT_NAME}"
CONTAINER_NAME="ai-dev-${PROJECT_NAME}"

if ! AGENT_DEFAULT_CMD=$(agent_command "$AGENT"); then
  echo "Unsupported agent: $AGENT"
  echo "Supported agents: claude, codex, gemini"
  exit 1
fi

AGENT_CMD="${AGENT_CMD:-$AGENT_DEFAULT_CMD}"

echo "Connecting to ${AGENT} container for project: ${PROJECT_NAME}"

if docker-compose -p "$COMPOSE_PROJECT_NAME" ps | grep -q "Up"; then
  docker-compose -p "$COMPOSE_PROJECT_NAME" exec ai-dev bash -lc "$AGENT_CMD"
else
  if docker ps --filter "name=${CONTAINER_NAME}" --format '{{.Names}}' | grep -q "${CONTAINER_NAME}"; then
    docker exec -it "${CONTAINER_NAME}" bash -lc "$AGENT_CMD"
  else
    echo "No running container found for project: ${PROJECT_NAME}"
    echo "Available containers:"
    docker ps --filter "name=ai-dev-" --format "  - {{.Names}}" || echo "  None"
    exit 1
  fi
fi

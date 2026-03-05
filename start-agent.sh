#!/bin/bash

set -euo pipefail

GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

show_help() {
  echo -e "${BLUE}🐳 AI Dev Docker Environment${NC}"
  echo "============================="
  echo ""
  echo "Usage: $0 [OPTIONS] [PROJECT_PATH] [PROJECT_NAME]"
  echo ""
  echo "Options:"
  echo "  -a, --agent AGENT   Agent to launch: claude | codex | gemini (default: claude)"
  echo "  -i, --interactive   Start and immediately connect (default)"
  echo "  -d, --detach        Start container in background only"
  echo "  -h, --help          Show this help message"
  echo ""
  echo "Arguments:"
  echo "  PROJECT_PATH        Path to project directory (default: current directory)"
  echo "  PROJECT_NAME        Container suffix/name (default: basename of project path)"
  echo ""
  echo "Env overrides:"
  echo "  PROJECT_PATH, PROJECT_NAME, AGENT"
  echo "  AGENT_CMD           Override launch command inside container"
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

INTERACTIVE=true
AGENT="${AGENT:-claude}"
POSITIONAL_ARGS=()

while [[ $# -gt 0 ]]; do
  case "$1" in
    -a|--agent)
      AGENT="$2"
      shift 2
      ;;
    -i|--interactive)
      INTERACTIVE=true
      shift
      ;;
    -d|--detach)
      INTERACTIVE=false
      shift
      ;;
    -h|--help)
      show_help
      exit 0
      ;;
    -*)
      echo -e "${RED}❌ Unknown option $1${NC}"
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

PROJECT_PATH="${1:-${PROJECT_PATH:-$(pwd)}}"
PROJECT_NAME="${2:-${PROJECT_NAME:-$(basename "$PROJECT_PATH")}}"
PROJECT_PATH=$(cd "$PROJECT_PATH" && pwd)

if [[ ! -d "$PROJECT_PATH" ]]; then
  echo -e "${RED}❌ Project path does not exist: $PROJECT_PATH${NC}"
  echo -e "${YELLOW}💡 Create the directory first or specify a different path.${NC}"
  exit 1
fi

if ! docker info > /dev/null 2>&1; then
  echo -e "${RED}❌ Docker is not running. Please start Docker and try again.${NC}"
  exit 1
fi

if ! AGENT_DEFAULT_CMD=$(agent_command "$AGENT"); then
  echo -e "${RED}❌ Unsupported agent: $AGENT${NC}"
  echo "Supported agents: claude, codex, gemini"
  exit 1
fi

AGENT_CMD="${AGENT_CMD:-$AGENT_DEFAULT_CMD}"

export PROJECT_PATH
export PROJECT_NAME
export AGENT
COMPOSE_PROJECT_NAME="ai-dev-${PROJECT_NAME}"
export COMPOSE_PROJECT_NAME

CONTAINER_NAME="ai-dev-${PROJECT_NAME}"

echo -e "${BLUE}🐳 AI Dev Docker Environment${NC}"
echo "============================="
echo -e "${BLUE}Project Path:${NC} $PROJECT_PATH"
echo -e "${BLUE}Project Name:${NC} $PROJECT_NAME"
echo -e "${BLUE}Agent:${NC} $AGENT"
echo ""

if [[ "$(docker ps -q -f name=${CONTAINER_NAME})" ]]; then
  echo -e "${GREEN}✅ Container already running, reusing existing container!${NC}"
else
  echo -e "${BLUE}🔨 Building container (cached layers reused when possible)...${NC}"
  docker-compose -p "$COMPOSE_PROJECT_NAME" build

  echo -e "${BLUE}🚀 Starting container...${NC}"
  docker-compose -p "$COMPOSE_PROJECT_NAME" up -d

  sleep 2
  echo -e "${GREEN}✅ Container started successfully!${NC}"
fi

echo ""
echo "Project mount:"
echo -e "${GREEN}  $PROJECT_PATH → /workspace/project${NC}"
echo ""

if [[ "$INTERACTIVE" == true ]]; then
  echo -e "${BLUE}🔗 Launching ${AGENT} in YOLO/unsafe mode...${NC}"
  echo ""
  docker-compose -p "$COMPOSE_PROJECT_NAME" exec ai-dev bash -lc "$AGENT_CMD"
else
  echo "Container running in background."
  echo ""
  echo "Connect later with:"
  echo -e "${GREEN}./connect.sh --agent $AGENT $PROJECT_NAME${NC}"
  echo ""
  echo "Or open a shell:"
  echo -e "${GREEN}docker exec -it ai-dev-${PROJECT_NAME} bash${NC}"
fi

#!/bin/bash

set -euo pipefail

if ! command -v ai-sandbox >/dev/null 2>&1; then
  echo "ai-sandbox is not installed."
  echo "Install it globally with:"
  echo "  go install github.com/leomorpho/yolo-ai-dev-mode/cmd/ai-sandbox@latest"
  exit 1
fi

exec ai-sandbox start "$@"

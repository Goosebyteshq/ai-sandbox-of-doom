#!/bin/bash

set -euo pipefail

if ! command -v doombox >/dev/null 2>&1; then
  echo "doombox is not installed."
  echo "Install it globally with:"
  echo "  go install github.com/Goosebyteshq/doombox/cmd/doombox@latest"
  exit 1
fi

exec doombox connect "$@"

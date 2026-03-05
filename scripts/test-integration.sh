#!/bin/bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

COMPOSE_PROJECT_NAME="ai-dev-integration"
CONTAINER_NAME="ai-dev-integration"

cleanup() {
  if [[ "${KEEP_INTEGRATION_CONTAINER:-0}" != "1" ]]; then
    PROJECT_PATH="${REPO_ROOT}" PROJECT_NAME="integration" AGENT=claude \
      docker-compose -p "${COMPOSE_PROJECT_NAME}" down >/dev/null 2>&1 || true
  fi
}

trap cleanup EXIT

echo "==> Building image (no cache)"
docker-compose build --no-cache

echo "==> Starting integration container"
PROJECT_PATH="${REPO_ROOT}" PROJECT_NAME="integration" AGENT=claude \
  docker-compose -p "${COMPOSE_PROJECT_NAME}" up -d

echo "==> Verifying installed CLIs and toolchain"
docker exec "${CONTAINER_NAME}" bash -lc '
  set -euo pipefail
  command -v claude
  command -v codex
  command -v gemini
  command -v go
  command -v node
  command -v pnpm
  claude --version
  codex --version
  gemini --version
  go version
  node --version
  pnpm --version
'

echo "==> Verifying YOLO/unsafe launch flags"
docker exec "${CONTAINER_NAME}" bash -lc '
  set -euo pipefail
  claude --dangerously-skip-permissions --help >/tmp/claude_help.txt
  codex --sandbox danger-full-access --ask-for-approval never --help >/tmp/codex_help.txt
  gemini --yolo --help >/tmp/gemini_help.txt
  wc -l /tmp/claude_help.txt /tmp/codex_help.txt /tmp/gemini_help.txt
'

echo "==> Running Go smoke test"
docker exec "${CONTAINER_NAME}" bash -lc '
  set -euo pipefail
  cat > /tmp/hello_test.go <<EOF
package main

import "testing"

func TestSum(t *testing.T) {
  if 2+2 != 4 {
    t.Fatalf("unexpected math")
  }
}
EOF
  go test /tmp/hello_test.go
'

echo "==> Running Playwright smoke test"
docker exec "${CONTAINER_NAME}" bash -lc '
  set -euo pipefail
  rm -rf /tmp/pw-smoke
  mkdir -p /tmp/pw-smoke
  cd /tmp/pw-smoke
  npm init -y >/dev/null
  npm i -D @playwright/test >/dev/null
  cat > smoke.spec.js <<EOF
const { test, expect } = require("@playwright/test");

test("basic page title", async ({ page }) => {
  await page.setContent("<title>ok</title><h1>Hello</h1>");
  await expect(page).toHaveTitle("ok");
});
EOF
  PLAYWRIGHT_BROWSERS_PATH=/ms-playwright npx playwright test --reporter=line
'

echo "==> Verifying mount isolation"
docker exec "${CONTAINER_NAME}" bash -lc '
  set -euo pipefail
  mount | grep "/workspace/project"
  mount | grep "/home/developer"
'

echo "Integration test suite passed."

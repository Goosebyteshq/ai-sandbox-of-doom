#!/bin/bash

set -euo pipefail

BIN_DIR="$(mktemp -d)"
BIN_PATH="${BIN_DIR}/doombox"
GO_MOD_CACHE="${BIN_DIR}/gomodcache"
GO_BUILD_CACHE="${BIN_DIR}/gocache"
trap 'rm -rf "${BIN_DIR}"' EXIT

GOMODCACHE="${GO_MOD_CACHE}" GOCACHE="${GO_BUILD_CACHE}" go build -o "${BIN_PATH}" ./cmd/doombox

ROOT_HELP="$("${BIN_PATH}" --help)"
HARNESS_HELP="$("${BIN_PATH}" harness help)"

echo "==> validate root help"
grep -q "doombox open" <<<"${ROOT_HELP}"
if grep -q "doombox start" <<<"${ROOT_HELP}"; then
  echo "unexpected deprecated command in root help: doombox start"
  exit 1
fi
if grep -q "doombox connect" <<<"${ROOT_HELP}"; then
  echo "unexpected deprecated command in root help: doombox connect"
  exit 1
fi

echo "==> validate harness help"
for expected in \
  "doombox harness init" \
  "doombox harness status" \
  "doombox harness score" \
  "doombox harness report" \
  "doombox harness export-eval" \
  "doombox harness compare" \
  "doombox harness flip"; do
  grep -q "${expected}" <<<"${HARNESS_HELP}"
done

echo "CLI smoke checks passed."

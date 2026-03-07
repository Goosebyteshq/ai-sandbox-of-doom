#!/bin/bash

set -euo pipefail

if grep -Eq '^[[:space:]]*replace[[:space:]].*=>[[:space:]]*\./' go.mod; then
  echo "root go.mod contains a local replace directive (=> ./...), which breaks go install @latest."
  echo "Use go.work for local module wiring instead."
  exit 1
fi

echo "No local replace directives found in root go.mod."

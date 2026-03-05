#!/bin/bash

# Backward-compatible wrapper
exec "$(dirname "$0")/start-agent.sh" --agent claude "$@"

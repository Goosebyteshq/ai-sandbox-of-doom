#!/bin/bash

exec "$(dirname "$0")/start-agent.sh" --agent gemini "$@"

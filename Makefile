SHELL := /bin/bash

.PHONY: install-cli build-cli test fast-check test-harness-sim test-integration

install-cli:
	go install ./cmd/doombox

build-cli:
	go build ./cmd/doombox

test:
	go test ./...

fast-check:
	./scripts/check-fast.sh

test-harness-sim:
	go test ./harness/adapters/mock

test-integration:
	./scripts/test-integration.sh

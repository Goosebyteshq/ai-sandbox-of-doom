SHELL := /bin/bash

.PHONY: install install-cli build-cli test fast-check test-cli-smoke test-harness-sim test-integration

install:
	go install ./cmd/doombox

install-cli:
	go install ./cmd/doombox

build-cli:
	go build ./cmd/doombox

test:
	go test ./...
	cd harness && go test ./...

fast-check:
	./scripts/check-fast.sh

test-harness-sim:
	cd harness && go test ./adapters/mock

test-cli-smoke:
	./scripts/test-cli-smoke.sh

test-integration:
	./scripts/test-integration.sh

SHELL := /bin/bash

.PHONY: install-cli build-cli test fast-check test-integration

install-cli:
	go install ./cmd/ai-sandbox

build-cli:
	go build ./cmd/ai-sandbox

test:
	go test ./...

fast-check:
	./scripts/check-fast.sh

test-integration:
	./scripts/test-integration.sh

SHELL := /bin/bash

.PHONY: install-cli build-cli test-integration

install-cli:
	go install ./cmd/ai-sandbox

build-cli:
	go build ./cmd/ai-sandbox

test-integration:
	./scripts/test-integration.sh

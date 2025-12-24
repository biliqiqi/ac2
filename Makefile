##
# ac2
#
# @file
# @version 0.1
SHELL := /bin/bash

.DEFAULT_GOAL := help

MAKE := make
GO := go
GO_LINT := golangci-lint
BINARY := ac2
BUILD_SRC := ./cmd/ac2

.PHONY: help lint build

help: ## Show helps
	@echo "help:"
	@echo "make [target]"
	@echo ""
	@echo "available targets:"
	@sed -ne '/@sed/!s/## //p' $(MAKEFILE_LIST)

lint: ## Lint Code
	@echo "Starting lint..."
	$(GO_LINT) run --issues-exit-code 0

build: ## Build
	@echo "Starting build..."
	$(GO) build -o $(BINARY) $(BUILD_SRC)

# end

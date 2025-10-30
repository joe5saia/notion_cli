SHELL := /bin/bash

GO ?= go

GOBIN := $(shell $(GO) env GOBIN)
ifeq ($(GOBIN),)
GOBIN := $(shell $(GO) env GOPATH)/bin
endif

GOFUMPT := $(GOBIN)/gofumpt
GOLANGCI_LINT := $(GOBIN)/golangci-lint

.PHONY: fmt fmt-check lint ci-lint tools

tools: $(GOFUMPT) $(GOLANGCI_LINT)

$(GOFUMPT):
	@echo "Installing gofumpt..."
	@$(GO) install mvdan.cc/gofumpt@v0.9.2

$(GOLANGCI_LINT):
	@echo "Installing golangci-lint..."
	@$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8

fmt: $(GOFUMPT)
	@files=$$(find . -name '*.go' -not -path './vendor/*' -not -path './.git/*'); \
	if [ -n "$$files" ]; then \
		"$(GOFUMPT)" -w $$files; \
	else \
		echo "No Go files to format"; \
	fi

fmt-check: $(GOFUMPT)
	@files=$$(find . -name '*.go' -not -path './vendor/*' -not -path './.git/*'); \
	if [ -z "$$files" ]; then \
		echo "No Go files to format"; \
	else \
		unformatted=$$("$(GOFUMPT)" -l $$files); \
		if [ -n "$$unformatted" ]; then \
			echo "$$unformatted"; \
			echo; \
			echo "Run 'make fmt' to apply gofumpt formatting."; \
			exit 1; \
		fi; \
	fi

lint: $(GOLANGCI_LINT)
	"$(GOLANGCI_LINT)" run

ci-lint: fmt-check lint

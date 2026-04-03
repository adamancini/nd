MODULE := github.com/RamXX/nd
BIN    := nd
PREFIX := $(HOME)/go/bin

PLUGIN_NAME     := nd
PLUGIN_SRC      := nd-skill
PLUGIN_DIR      := $(shell pwd)/$(PLUGIN_SRC)

.PHONY: help build test test-e2e vet install update install-plugin install-skill uninstall-plugin clean

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  %-15s %s\n", $$1, $$2}'

VERSION := $(shell git describe --tags --always 2>/dev/null || echo dev)
# Strip leading 'v' for plugin directory (v0.6.0 -> 0.6.0)
PLUGIN_VERSION := $(shell echo $(VERSION) | sed 's/^v//')

build: ## Build nd binary
	go build -ldflags "-X github.com/RamXX/nd/cmd.version=$(VERSION)" -o $(BIN) .

test: ## Run tests
	go test -v ./...

test-e2e: ## Run e2e tests for plugin lifecycle
	@bash e2e/plugin_lifecycle_test.sh

vet: ## Run go vet
	go vet ./...

install: build install-plugin ## Install nd binary and Claude Code plugin
	mkdir -p $(PREFIX)
	cp $(BIN) $(PREFIX)/$(BIN)

update: build ## Update nd binary and Claude Code plugin
	mkdir -p $(PREFIX)
	cp $(BIN) $(PREFIX)/$(BIN)
	@claude plugin marketplace update "$(PLUGIN_NAME)"
	@claude plugin update "$(PLUGIN_NAME)@$(PLUGIN_NAME)"
	@echo "  nd plugin updated to $(PLUGIN_VERSION) -- restart Claude Code to activate"

install-plugin: ## Install Claude Code plugin to ~/.claude/plugins
	@claude plugin marketplace add "$(PLUGIN_DIR)" 2>/dev/null \
		&& echo "  Marketplace registered." \
		|| echo "  Marketplace already registered."
	@claude plugin install "$(PLUGIN_NAME)@$(PLUGIN_NAME)" 2>/dev/null \
		&& echo "  Plugin installed." \
		|| echo "  Plugin already installed -- run 'make update' to pick up changes."
	@echo "  nd plugin installed -- restart Claude Code to activate"

install-skill: install-plugin ## Alias for install-plugin (matches vlt convention)

uninstall-plugin: ## Remove Claude Code plugin
	@claude plugin uninstall "$(PLUGIN_NAME)@$(PLUGIN_NAME)" 2>/dev/null \
		&& echo "  Plugin uninstalled." \
		|| echo "  Plugin was not installed."
	@claude plugin marketplace remove "$(PLUGIN_NAME)" 2>/dev/null \
		&& echo "  Marketplace removed." \
		|| echo "  Marketplace was not registered."
	@echo "  nd plugin uninstalled"

clean: ## Remove build artifacts
	rm -f $(BIN)

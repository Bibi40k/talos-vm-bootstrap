.PHONY: help build build-cli test test-v test-cover test-cover-all lint fmt vet vulncheck clean deps verify install install-requirements install-vmbootstrap update-vmbootstrap-pin config run run-dry vm-deploy talos-bootstrap talos-bootstrap-dry run-workflow cluster-status mount-check kubeconfig-export check-go

# Auto-download Go toolchain if local version < go.mod requirement
export GOTOOLCHAIN=auto

SHELL := /bin/bash

# ANSI colors (actual ESC char via printf - works in both printf and sed)
ESC    := $(shell printf '\033')
BOLD   := $(ESC)[1m
CYAN   := $(ESC)[36m
GREEN  := $(ESC)[32m
YELLOW := $(ESC)[33m
RED    := $(ESC)[31m
GREY   := $(ESC)[90m
RESET  := $(ESC)[0m

# Ensure local Go toolchain meets go.mod requirement (avoids toolchain auto confusion)
GO_REQUIRED := $(shell awk '/^go /{print $$2}' go.mod)
CONFIG ?= configs/talos-bootstrap.yaml
BOOTSTRAP_RESULT ?=
VM_CONFIG ?=
DRY ?= 0
FORCE ?= 0
VMBOOTSTRAP_BIN ?= bin/vmbootstrap
VMBOOTSTRAP_AUTO_BUILD ?= false
VMBOOTSTRAP_UPDATE_NOTIFY ?= true
VMBOOTSTRAP_REPO ?= ../vmware-vm-bootstrap
VMBOOTSTRAP_GOPROXY ?= direct

# Run linter (requires golangci-lint)
GOLANGCI_LINT := $(shell command -v golangci-lint 2>/dev/null)
ifeq ($(GOLANGCI_LINT),)
GOLANGCI_LINT := $(shell GOPATH=$$(go env GOPATH); [ -x "$$GOPATH/bin/golangci-lint" ] && echo "$$GOPATH/bin/golangci-lint")
endif
ifeq ($(GOLANGCI_LINT),)
GOLANGCI_LINT := $(shell [ -x "/usr/local/bin/golangci-lint" ] && echo "/usr/local/bin/golangci-lint")
endif

# Run govulncheck (requires govulncheck)
GO_BIN_DIR := $(shell [ -x "/usr/local/go/bin/go" ] && echo "/usr/local/go/bin")
GOVULNCHECK := $(shell GOPATH=$$(go env GOPATH); [ -x "$$GOPATH/bin/govulncheck" ] && echo "$$GOPATH/bin/govulncheck")
ifeq ($(GOVULNCHECK),)
GOVULNCHECK := $(shell [ -x "/usr/local/bin/govulncheck" ] && echo "/usr/local/bin/govulncheck")
endif
ifeq ($(GOVULNCHECK),)
GOVULNCHECK := $(shell command -v govulncheck 2>/dev/null)
endif
GO_LOCAL_VERSION := $(shell GOTOOLCHAIN=local go env GOVERSION 2>/dev/null | sed 's/^go//')
GO_LOCAL_OK := $(shell [ -n "$(GO_LOCAL_VERSION)" ] && [ "$$(printf '%s\n%s\n' "$(GO_REQUIRED)" "$(GO_LOCAL_VERSION)" | sort -V | head -n1)" = "$(GO_REQUIRED)" ] && echo "yes")

# Default target
help:
	@printf "\n$(BOLD)talos-vm-bootstrap$(RESET)\n"
	@printf "$(CYAN)━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━$(RESET)\n"
	@printf "\n$(BOLD)  Development$(RESET)\n"
	@printf "    $(GREEN)make build$(RESET)			Compile check - verify all packages compile\n"
	@printf "    $(GREEN)make fmt$(RESET)			Auto-format source files\n"
	@printf "    $(GREEN)make vet$(RESET)			Detect suspicious code patterns (silence = OK)\n"
	@printf "    $(GREEN)make lint$(RESET)			Deep linting via golangci-lint (superset of vet)\n"
	@printf "\n$(BOLD)  Setup$(RESET)\n"
	@printf "    $(GREEN)make install-requirements$(RESET)  	Install all required external tools\n"
	@printf "    $(GREEN)make install-vmbootstrap$(RESET)   	Install pinned vmbootstrap CLI from go.mod\n"
	@printf "\n$(BOLD)  Testing$(RESET)\n"
	@printf "    $(GREEN)make test$(RESET)          		Run all tests\n"
	@printf "    $(GREEN)make test-v$(RESET)        		Run tests with verbose output\n"
	@printf "    $(GREEN)make test-cover$(RESET)    		Run core coverage gate (>80%% target)\n"
	@printf "    $(GREEN)make test-cover-all$(RESET)		Run full-package coverage report\n"
	@printf "    $(GREEN)make vulncheck$(RESET)     		Run govulncheck security scan\n"
	@printf "\n$(BOLD)  VM Management $(YELLOW)(requires configs/vcenter.sops.yaml)$(RESET)\n"
	@printf "    $(GREEN)make config$(RESET)            	Interactive config manager (create/edit VM configs)\n"
	@printf "    $(GREEN)make vm-deploy$(RESET)         	Select a VM config and bootstrap it\n"
	@printf "\n$(BOLD)  Talos Management $(YELLOW)(requires configs/talos-bootstrap.yaml)$(RESET)\n"
	@printf "    $(GREEN)make config$(RESET)            	Alias to config manager (also prepares Talos bootstrap config)\n"
	@printf "    $(GREEN)make talos-bootstrap$(RESET)   	Run Talos bootstrap (Docker + Talos), set DRY=1 for dry-run\n"
	@printf "    $(GREEN)make run-workflow$(RESET)      	Advanced (CI/pipeline): run orchestrated flow (optional BOOTSTRAP_RESULT)\n"
	@printf "    $(GREEN)make cluster-status$(RESET)    	Show remote Talos cluster status\n"
	@printf "    $(GREEN)make mount-check$(RESET)       	Verify mount path visibility in Talos node\n"
	@printf "    $(GREEN)make kubeconfig-export$(RESET)	Export kubeconfig to OUT=...\n"
	@printf "\n$(BOLD)  Maintenance$(RESET)\n"
	@printf "    $(GREEN)make clean$(RESET)			Remove build artifacts and caches\n"
	@printf "    $(GREEN)make deps$(RESET)			Download + tidy dependencies\n"
	@printf "    $(GREEN)make verify$(RESET)			Verify dependency checksums\n"
	@printf "\n$(CYAN)━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━$(RESET)\n\n"

check-go:
	@{ \
		LOCAL_VER=$$(GOTOOLCHAIN=local go env GOVERSION 2>/dev/null | sed 's/^go//' || echo "unknown"); \
		REQUIRED="$(GO_REQUIRED)"; \
		if [ "$$LOCAL_VER" = "unknown" ]; then \
			printf "$(YELLOW)Go not found in PATH (local toolchain).$(RESET)\n"; \
			printf "  Run: $(GREEN)make install-requirements$(RESET)\n\n"; \
			exit 1; \
		fi; \
		LOWEST=$$(printf '%s\n%s\n' "$$REQUIRED" "$$LOCAL_VER" | sort -V | head -n1); \
		if [ "$$LOWEST" != "$$REQUIRED" ]; then \
			printf "$(YELLOW)Local Go toolchain too old (local: $$LOCAL_VER, required: $$REQUIRED).$(RESET)\n"; \
			printf "  Run: $(GREEN)make install-requirements$(RESET)\n\n"; \
			exit 1; \
		fi; \
	}

build: check-go
	@printf "$(CYAN)Compiling all packages...$(RESET)\n"
	@go build ./...
	@printf "$(GREEN)✓ All packages compile OK$(RESET)\n"

build-cli: check-go
	@printf "$(CYAN)Building talos-vm-bootstrap...$(RESET)\n"
	@mkdir -p bin
	@go build -o bin/talos-vm-bootstrap ./cmd/talos-vm-bootstrap
	@printf "$(GREEN)✓ Built: bin/talos-vm-bootstrap$(RESET)\n"

test: check-go
	@printf "$(CYAN)Running tests...$(RESET)\n"
	@set -o pipefail; go test ./... 2>&1 | sed \
		-e 's|^ok |$(GREEN)ok$(RESET) |g' \
		-e 's|^FAIL|$(RED)FAIL$(RESET)|g' \
		-e 's|^?|$(GREY)?|g' \
		-e 's|\[no test files\]|$(GREY)[no test files]$(RESET)|g'

test-v: check-go
	@printf "$(CYAN)Running tests (verbose)...$(RESET)\n"
	@set -o pipefail; go test -v ./... 2>&1 | sed \
		-e 's|^ok |$(GREEN)ok$(RESET) |g' \
		-e 's|^FAIL|$(RED)FAIL$(RESET)|g' \
		-e 's|^PASS$$|$(GREEN)PASS$(RESET)|g' \
		-e 's|^--- PASS|$(GREEN)--- PASS$(RESET)|g' \
		-e 's|^    --- PASS|    $(GREEN)--- PASS$(RESET)|g' \
		-e 's|^--- FAIL|$(RED)--- FAIL$(RESET)|g' \
		-e 's|^    --- FAIL|    $(RED)--- FAIL$(RESET)|g' \
		-e 's|^--- SKIP|$(YELLOW)--- SKIP$(RESET)|g' \
		-e 's|^    --- SKIP|    $(YELLOW)--- SKIP$(RESET)|g' \
		-e 's|^=== RUN|$(GREY)=== RUN$(RESET)|g' \
		-e 's|^    === RUN|    $(GREY)=== RUN$(RESET)|g' \
		-e 's|^?|$(GREY)?|g' \
		-e 's|\[no test files\]|$(GREY)[no test files]$(RESET)|g'

# Core production logic packages (coverage gate)
COVER_PKGS := ./internal/bootstrap ./internal/config ./internal/workflow

test-cover: check-go
	@printf "$(CYAN)Running core coverage tests...$(RESET)\n"
	@mkdir -p tmp
	@set -o pipefail; \
	GOCOVERDIR= go test -coverprofile=tmp/coverage.out $(COVER_PKGS) 2>&1 | sed \
		-e 's|^ok |$(GREEN)ok$(RESET) |g' \
		-e 's|^FAIL|$(RED)FAIL$(RESET)|g' \
		-e 's|^?|$(GREY)?|g' \
		-e 's|\[no test files\]|$(GREY)[no test files]$(RESET)|g'; \
	go tool cover -html=tmp/coverage.out -o tmp/coverage.html
	@printf "$(GREEN)✓ Coverage report: tmp/coverage.html$(RESET)\n"

test-cover-all: check-go
	@printf "$(CYAN)Running full-package coverage...$(RESET)\n"
	@mkdir -p tmp
	@set -o pipefail; \
	PKGS=$$(go list ./... | grep -v '/tools'); \
	GOCOVERDIR= go test -coverprofile=tmp/coverage.all.out $$PKGS 2>&1 | sed \
		-e 's|^ok |$(GREEN)ok$(RESET) |g' \
		-e 's|^FAIL|$(RED)FAIL$(RESET)|g' \
		-e 's|^?|$(GREY)?|g' \
		-e 's|\[no test files\]|$(GREY)[no test files]$(RESET)|g'; \
	go tool cover -html=tmp/coverage.all.out -o tmp/coverage.all.html
	@printf "$(GREEN)✓ Full coverage report: tmp/coverage.all.html$(RESET)\n"

lint:
	@{ \
		if [ -z "$(GOLANGCI_LINT)" ]; then \
			printf "$(YELLOW)golangci-lint not installed.$(RESET)\n"; \
			printf "  Run: $(GREEN)make install-requirements$(RESET)\n\n"; \
			exit 0; \
		fi; \
	}; \
	printf "$(CYAN)Running golangci-lint...$(RESET)\n"; \
	"$(GOLANGCI_LINT)" run ./...

fmt:
	@printf "$(CYAN)Formatting code...$(RESET)\n"
	@go fmt ./...
	@printf "$(GREEN)✓ Done$(RESET)\n"

vulncheck:
	@{ \
		if [ -z "$(GOVULNCHECK)" ]; then \
			printf "$(YELLOW)govulncheck not installed.$(RESET)\n"; \
			printf "  Run: $(GREEN)make install-requirements$(RESET)\n\n"; \
			exit 0; \
		fi; \
	}; \
	printf "$(CYAN)Running govulncheck...$(RESET)\n"; \
	if [ "$(GO_LOCAL_OK)" = "yes" ]; then \
		PATH="$(GO_BIN_DIR):$$PATH" GOTOOLCHAIN=local "$(GOVULNCHECK)" ./...; \
	else \
		PATH="$(GO_BIN_DIR):$$PATH" GOTOOLCHAIN=auto "$(GOVULNCHECK)" ./...; \
	fi

vet: check-go
	@printf "$(CYAN)Running go vet...$(RESET)\n"
	@go vet ./...
	@printf "$(GREEN)✓ Done$(RESET)\n"

clean:
	@printf "$(CYAN)Cleaning build artifacts...$(RESET)\n"
	@rm -f tmp/coverage.out tmp/coverage.html tmp/coverage.all.out tmp/coverage.all.html bin/talos-vm-bootstrap
	@go clean -testcache
	@printf "$(GREEN)✓ Clean complete$(RESET)\n"

deps: check-go
	@printf "$(CYAN)Updating dependencies...$(RESET)\n"
	@go mod download
	@go mod tidy
	@printf "$(GREEN)✓ Dependencies updated$(RESET)\n"

verify: check-go
	@printf "$(CYAN)Verifying dependencies...$(RESET)\n"
	@go mod verify
	@printf "$(GREEN)✓ All dependencies verified$(RESET)\n"

install-requirements:
	@bash scripts/install-requirements.sh

install-vmbootstrap: check-go
	@printf "$(CYAN)Installing vmbootstrap (pinned from go.mod)...$(RESET)\n"
	@GOPROXY="$(VMBOOTSTRAP_GOPROXY)" go run ./tools/vmbootstrapctl install --dir "$(CURDIR)/bin" >/tmp/vmbootstrap-install.out
	@printf "$(GREEN)✓ %s$(RESET)\n" "$$(cat /tmp/vmbootstrap-install.out)"
	@rm -f /tmp/vmbootstrap-install.out
	@$(MAKE) vmbootstrap-sync-assets FORCE="$(FORCE)"

update-vmbootstrap-pin: check-go
	@printf "$(CYAN)Updating vmbootstrap module pin to latest...$(RESET)\n"
	@NEW=$$(GOPROXY="$(VMBOOTSTRAP_GOPROXY)" go run ./tools/vmbootstrapctl update-pin); \
	printf "$(GREEN)✓ Updated pin. New version: %s$(RESET)\n" "$$NEW"
	@$(MAKE) vmbootstrap-sync-assets FORCE="$(FORCE)"

vmbootstrap-sync-assets: check-go
	@printf "$(CYAN)Syncing vmbootstrap assets from pinned module...$(RESET)\n"
	@FORCE_FLAG=""; \
	if [ "$(FORCE)" = "1" ] || [ "$(FORCE)" = "true" ]; then FORCE_FLAG="--force"; fi; \
	GOPROXY="$(VMBOOTSTRAP_GOPROXY)" go run ./tools/vmbootstrapctl sync-assets --repo-root "$(CURDIR)" $$FORCE_FLAG

install: check-go
	@printf "$(CYAN)Installing talos-vm-bootstrap CLI...$(RESET)\n"
	@go install ./cmd/talos-vm-bootstrap
	@printf "$(GREEN)✓ Installed! Run: talos-vm-bootstrap$(RESET)\n"

talos-bootstrap-dry: build-cli
	@$(MAKE) talos-bootstrap DRY=1

config: build-cli
	@bin/talos-vm-bootstrap config \
		--config "$(CONFIG)" \
		--vmbootstrap-bin "$(VMBOOTSTRAP_BIN)" \
		--vmbootstrap-repo "$(VMBOOTSTRAP_REPO)" \
		--vmbootstrap-auto-build="$(VMBOOTSTRAP_AUTO_BUILD)" \
		--vmbootstrap-update-notify="$(VMBOOTSTRAP_UPDATE_NOTIFY)"

vm-deploy: build-cli
	@bin/talos-vm-bootstrap vm-deploy \
		--vmbootstrap-bin "$(VMBOOTSTRAP_BIN)" \
		--vmbootstrap-repo "$(VMBOOTSTRAP_REPO)" \
		--vmbootstrap-auto-build="$(VMBOOTSTRAP_AUTO_BUILD)" \
		--vmbootstrap-update-notify="$(VMBOOTSTRAP_UPDATE_NOTIFY)" \
		$(if $(BOOTSTRAP_RESULT),--bootstrap-result "$(BOOTSTRAP_RESULT)",)

run: vm-deploy

talos-bootstrap: build-cli
	@if [ ! -f "$(CONFIG)" ]; then \
		printf "\n  $(YELLOW)No Talos bootstrap config found: $(CONFIG)$(RESET)\n"; \
		printf "  Run: $(GREEN)make config$(RESET) → Create missing config files\n\n"; \
		exit 1; \
	fi
	@DRY_FLAG=""; \
	if [ "$(DRY)" = "1" ]; then DRY_FLAG="--dry-run"; fi; \
	bin/talos-vm-bootstrap bootstrap --config "$(CONFIG)" $$DRY_FLAG

run-dry: talos-bootstrap-dry

run-workflow: build-cli
	@if [ ! -f "$(CONFIG)" ]; then \
		printf "\n  $(YELLOW)Missing Talos bootstrap config: $(CONFIG)$(RESET)\n"; \
		printf "  Run: $(GREEN)make config$(RESET)\n\n"; \
		exit 1; \
	fi; \
	VM_CONFIG_FLAG=""; \
	if [ -n "$(VM_CONFIG)" ]; then VM_CONFIG_FLAG="--vm-config $(VM_CONFIG)"; fi; \
	BOOTSTRAP_FLAG=""; \
	if [ -n "$(BOOTSTRAP_RESULT)" ]; then BOOTSTRAP_FLAG="--bootstrap-result $(BOOTSTRAP_RESULT)"; fi; \
	bin/talos-vm-bootstrap provision-and-bootstrap --config "$(CONFIG)" $$VM_CONFIG_FLAG $$BOOTSTRAP_FLAG \
		--vmbootstrap-bin "$(VMBOOTSTRAP_BIN)" \
		--vmbootstrap-repo "$(VMBOOTSTRAP_REPO)" \
		--vmbootstrap-auto-build="$(VMBOOTSTRAP_AUTO_BUILD)" \
		--vmbootstrap-update-notify="$(VMBOOTSTRAP_UPDATE_NOTIFY)"

cluster-status: build-cli
	@if [ ! -f "$(CONFIG)" ]; then \
		printf "\n  $(YELLOW)Missing Talos bootstrap config: $(CONFIG)$(RESET)\n"; \
		printf "  Run: $(GREEN)make config$(RESET)\n\n"; \
		exit 1; \
	fi
	@bin/talos-vm-bootstrap cluster-status --config "$(CONFIG)"

mount-check: build-cli
	@if [ ! -f "$(CONFIG)" ]; then \
		printf "\n  $(YELLOW)Missing Talos bootstrap config: $(CONFIG)$(RESET)\n"; \
		printf "  Run: $(GREEN)make config$(RESET)\n\n"; \
		exit 1; \
	fi
	@bin/talos-vm-bootstrap mount-check --config "$(CONFIG)"

kubeconfig-export: build-cli
	@if [ ! -f "$(CONFIG)" ]; then \
		printf "\n  $(YELLOW)Missing Talos bootstrap config: $(CONFIG)$(RESET)\n"; \
		printf "  Run: $(GREEN)make config$(RESET)\n\n"; \
		exit 1; \
	fi; \
	if [ -z "$(OUT)" ]; then \
		printf "\n  $(YELLOW)Missing OUT for kubeconfig export$(RESET)\n"; \
		printf "  Example: $(GREEN)make kubeconfig-export OUT=build/devvm/kubeconfig$(RESET)\n\n"; \
		exit 1; \
	fi
	@bin/talos-vm-bootstrap kubeconfig-export --config "$(CONFIG)" --out "$(OUT)"

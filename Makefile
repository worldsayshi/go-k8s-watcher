# Makefile for go-k8s-watcher
# Organizes functionality using external scripts and manifests

# Colors for better output
GREEN := \033[0;32m
YELLOW := \033[1;33m
BLUE := \033[0;34m
RED := \033[0;31m
NC := \033[0m  # No Color

.PHONY: help
help:
	@echo "Available targets:"
	@echo "  help           - Show this help message"
	@echo "  prereqs        - Check prerequisites"
	@echo "  create-cluster - Create Kind cluster for testing"
	@echo "  resources      - Create test resources in the cluster"
	@echo "  modify         - Modify resources to trigger events"
	@echo "  examples       - Show example commands"
	@echo "  cleanup        - Delete the Kind cluster"
	@echo "  start-watcher  - Start the Kubernetes resource watcher"
	@echo "  start-tui      - Start the Kubernetes resource TUI"
	@echo "  build          - Build both commands"
	@echo "  test-only      - Run test sequence (resources & modify) without starting watcher"
	@echo "  e2e-test       - Run end-to-end test with automatic watcher start/stop"

.PHONY: prereqs
prereqs:
	@echo -e "${BLUE}========== Checking Prerequisites ==========${NC}\n"
	@./scripts/check_prerequisites.sh

.PHONY: create-cluster
create-cluster: prereqs
	@echo -e "${BLUE}========== Setting up Kind Cluster ==========${NC}\n"
	@./scripts/create_cluster.sh

.PHONY: resources
resources:
	@echo -e "${BLUE}========== Creating Test Resources ==========${NC}\n"
	@./scripts/create_resources.sh

.PHONY: modify
modify:
	@echo -e "${BLUE}========== Modifying Resources to Generate Events ==========${NC}\n"
	@./scripts/modify_resources.sh

.PHONY: examples
examples:
	@echo -e "${BLUE}========== Example Commands for the Kubernetes Resource Watcher ==========${NC}\n"
	@./scripts/show_examples.sh

.PHONY: cleanup
cleanup:
	@echo -e "${BLUE}========== Cleaning up Resources ==========${NC}\n"
	@./scripts/cleanup.sh

# Start the Kubernetes resource watcher in the background
.PHONY: start-watcher
start-watcher:
	@echo -e "${BLUE}========== Starting Resource Watcher ==========${NC}\n"
	@echo -e "${YELLOW}Starting the resource watcher for namespace: test-ns-1${NC}"
	@go run cmd/watcher/main.go --all --namespace=test-ns-1

# Start the TUI application
.PHONY: start-tui
start-tui:
	@echo -e "${BLUE}========== Starting Resource TUI ==========${NC}\n"
	@echo -e "${YELLOW}Starting the resource TUI viewer${NC}"
	@LOG_PATH="/tmp/k8s-tui.log" && \
	echo -e "${GREEN}Logs will be written to: $${LOG_PATH}${NC}" && \
	go run cmd/tui/main.go --log="$${LOG_PATH}"

# Build both commands
.PHONY: build
build:
	@echo -e "${BLUE}========== Building Commands ==========${NC}\n"
	@go build -o bin/watcher cmd/watcher/main.go
	@go build -o bin/tui cmd/tui/main.go
	@echo -e "${GREEN}✓ Built commands in bin/ directory${NC}"

# Run test sequence without starting watcher
.PHONY: test-only
test-only: create-cluster
	@echo -e "${BLUE}========== Running Test Only (No Watcher) ==========${NC}\n"
	@./scripts/run_e2e_test.sh

# End-to-end test target that runs the full sequence with automatic watcher
.PHONY: e2e-test
e2e-test: create-cluster
	@echo -e "${BLUE}========== Running End-to-End Test with Watcher ==========${NC}\n"
	@echo -e "${YELLOW}Starting the resource watcher in background...${NC}"
	@go run cmd/watcher/main.go --all --namespace=test-ns-1 > watcher-output.log 2>&1 & \
	WATCHER_PID=$$!; \
	echo "Watcher started with PID: $$WATCHER_PID"; \
	sleep 5; \
	./scripts/run_e2e_test.sh $$WATCHER_PID

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
	@echo "  modify         - Modify resources to trigger watch events"
	@echo "  examples       - Show example commands"
	@echo "  cleanup        - Delete the Kind cluster"
	@echo "  e2e-test       - Run end-to-end test without interruptions"

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

# End-to-end test target that runs the full sequence without interactive prompts
.PHONY: e2e-test
e2e-test: create-cluster
	@echo -e "${BLUE}========== Running End-to-End Test ==========${NC}\n"
	@./scripts/run_e2e_test.sh

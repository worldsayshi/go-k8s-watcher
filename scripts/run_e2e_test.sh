#!/bin/bash
# run_e2e_test.sh - Run end-to-end test without interactive prompts

# Colors for better output
BLUE='\033[0;34m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${BLUE}========== Running End-to-End Test ==========${NC}\n"

echo -e "${YELLOW}Starting the resource watcher in background...${NC}"
# Start the watcher in the background and save its output
go run main.go --all --namespace=test-ns-1 > watcher-output.log 2>&1 &
WATCHER_PID=$!
echo "Watcher started with PID: $WATCHER_PID"
sleep 5

echo -e "${YELLOW}Creating test resources...${NC}"
./scripts/create_resources.sh
sleep 5

echo -e "${YELLOW}Modifying resources to trigger events...${NC}"
./scripts/modify_resources.sh
sleep 10

echo -e "${YELLOW}Stopping the watcher...${NC}"
kill $WATCHER_PID || true

echo -e "${BLUE}========== Test Results ==========${NC}"
echo "Watcher log content:"
cat watcher-output.log | tail -n 20
echo

echo -e "${YELLOW}Do you want to clean up? (Cluster will remain for inspection)${NC}"
echo "Run 'make cleanup' to remove the cluster when done"

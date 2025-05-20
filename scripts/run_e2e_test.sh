#!/bin/bash
# run_e2e_test.sh - Run end-to-end test without interactive prompts
#
# Usage: run_e2e_test.sh [watcher_pid]
# If watcher_pid is provided, the script will assume a watcher is already running
# with that PID. If not provided, no watcher will be started or stopped.

# Colors for better output
BLUE='\033[0;34m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${BLUE}========== Running End-to-End Test ==========${NC}\n"

# Check if a watcher PID was passed
WATCHER_PID=$1_e2e_test.sh - Run end-to-end test without interactive prompts
#
# Usage: run_e2e_test.sh [watcher_pid]
# If watcher_pid is provided, the script will assume a watcher is already running
# with that PID. If not provided, no watcher will be started or stopped.

# Colors for better output
BLUE='\033[0;34m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${BLUE}========== Running End-to-End Test ==========${NC}\n"

# Check if a watcher PID was passed
WATCHER_PID=$1

echo -e "${YELLOW}Creating test resources...${NC}"
./scripts/create_resources.sh
sleep 5

echo -e "${YELLOW}Modifying resources to trigger events...${NC}"
./scripts/modify_resources.sh
sleep 10

# Only output watcher info and attempt to stop if a watcher PID was provided
if [ -n "$WATCHER_PID" ]; then
    echo -e "${YELLOW}Stopping the watcher...${NC}"
    kill $WATCHER_PID || true

    echo -e "${BLUE}========== Test Results ==========${NC}"
    echo "Watcher log content:"
    cat watcher-output.log | tail -n 20
    echo
fi

echo -e "${YELLOW}Test complete. Cluster will remain for inspection.${NC}"
echo "Run 'make cleanup' to remove the cluster when done"

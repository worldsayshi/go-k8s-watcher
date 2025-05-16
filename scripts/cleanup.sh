#!/bin/bash
# cleanup.sh - Delete the Kind cluster

# Colors for better output
GREEN='\033[0;32m'
NC='\033[0m' # No Color

echo "Deleting Kind cluster 'k8s-watch-test'..."
kind delete cluster --name k8s-watch-test
echo -e "${GREEN}✓ Cluster deleted${NC}"

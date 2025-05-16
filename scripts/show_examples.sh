#!/bin/bash
# show_examples.sh - Show example commands for the watcher

# Colors for better output
GREEN='\033[0;32m'
NC='\033[0m' # No Color

echo -e "${GREEN}Watch pods in test-ns-1:${NC}"
echo "go run main.go --kind=Pod --api-version=v1 --namespace=test-ns-1"
echo

echo -e "${GREEN}Watch all resources in test-ns-1:${NC}"
echo "go run main.go --all --namespace=test-ns-1"
echo

echo -e "${GREEN}Watch deployments across all namespaces:${NC}"
echo "go run main.go --kind=Deployment --api-version=apps/v1 --all-namespaces"
echo

echo -e "${GREEN}Watch custom resources:${NC}"
echo "go run main.go --kind=App --api-version=example.com/v1 --namespace=test-ns-1"
echo

echo -e "${GREEN}Watch everything (might be noisy):${NC}"
echo "go run main.go --all --all-namespaces"

#!/bin/bash
# check_prerequisites.sh - Check if required tools are installed

# Colors for better output
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Check for kind
if ! command -v kind &> /dev/null; then
  echo -e "${RED}✗ kind is not installed. Please install it: https://kind.sigs.k8s.io/docs/user/quick-start/${NC}"
  exit 1
fi
echo -e "${GREEN}✓ kind is installed${NC}"

# Check for kubectl
if ! command -v kubectl &> /dev/null; then
  echo -e "${RED}✗ kubectl is not installed. Please install it: https://kubernetes.io/docs/tasks/tools/install-kubectl/${NC}"
  exit 1
fi
echo -e "${GREEN}✓ kubectl is installed${NC}"

# Check for go
if ! command -v go &> /dev/null; then
  echo -e "${RED}✗ go is not installed. Please install it: https://golang.org/doc/install${NC}"
  exit 1
fi
echo -e "${GREEN}✓ go is installed${NC}"

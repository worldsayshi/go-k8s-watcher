#!/bin/bash
# create_cluster.sh - Create Kind cluster for testing

# Colors for better output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Check if the cluster already exists
if kind get clusters | grep -q "k8s-watch-test"; then
  echo -e "${YELLOW}! Cluster 'k8s-watch-test' already exists. Using the existing cluster.${NC}"
else
  echo "Creating Kind cluster 'k8s-watch-test'..."
  kind create cluster --config manifests/kind-config.yaml
  echo -e "${GREEN}✓ Cluster 'k8s-watch-test' created successfully${NC}"
fi

echo "Setting kubectl context to the kind cluster..."
kubectl config use-context kind-k8s-watch-test

echo "Waiting for the cluster to be ready..."
kubectl wait --for=condition=Ready nodes --all --timeout=60s

echo -e "${GREEN}✓ Kind cluster is ready${NC}"

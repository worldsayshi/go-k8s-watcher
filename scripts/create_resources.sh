#!/bin/bash
# create_resources.sh - Create test resources in the cluster

# Colors for better output
GREEN='\033[0;32m'
NC='\033[0m' # No Color

echo "Creating test namespaces..."
kubectl create namespace test-ns-1 --dry-run=client -o yaml | kubectl apply -f -
kubectl create namespace test-ns-2 --dry-run=client -o yaml | kubectl apply -f -
echo -e "${GREEN}✓ Created test namespaces${NC}"

echo "Creating ConfigMaps..."
kubectl create configmap test-config-1 --from-literal=key1=value1 -n test-ns-1 --dry-run=client -o yaml | kubectl apply -f -
kubectl create configmap test-config-2 --from-literal=key2=value2 -n test-ns-2 --dry-run=client -o yaml | kubectl apply -f -
echo -e "${GREEN}✓ Created ConfigMaps${NC}"

echo "Creating Secrets..."
kubectl create secret generic test-secret-1 --from-literal=password=secret123 -n test-ns-1 --dry-run=client -o yaml | kubectl apply -f -
echo -e "${GREEN}✓ Created Secrets${NC}"

echo "Creating Deployments..."
kubectl apply -f manifests/deployments.yaml
echo -e "${GREEN}✓ Created Deployments${NC}"

echo "Creating Services..."
kubectl expose deployment nginx-deployment --port=80 -n test-ns-1 --dry-run=client -o yaml | kubectl apply -f -
kubectl expose deployment redis-deployment --port=6379 -n test-ns-2 --dry-run=client -o yaml | kubectl apply -f -
echo -e "${GREEN}✓ Created Services${NC}"

echo "Creating a Custom Resource Definition..."
kubectl apply -f manifests/app-crd.yaml
echo -e "${GREEN}✓ Created CRD${NC}"

echo "Creating a Custom Resource..."
echo "Waiting for CRD to be established..."
sleep 5
kubectl apply -f manifests/app-cr.yaml
echo -e "${GREEN}✓ Created Custom Resource${NC}"

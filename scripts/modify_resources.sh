#!/bin/bash
# modify_resources.sh - Modify resources to trigger watch events

# Colors for better output
GREEN='\033[0;32m'
NC='\033[0m' # No Color

echo "Updating ConfigMap..."
kubectl patch configmap test-config-1 -n test-ns-1 --type=merge -p '{"data":{"key1":"updated-value1","new-key":"new-value"}}'
echo -e "${GREEN}✓ Updated ConfigMap${NC}"

echo "Scaling Deployment..."
kubectl scale deployment nginx-deployment -n test-ns-1 --replicas=2
echo -e "${GREEN}✓ Scaled Deployment${NC}"

echo "Updating Custom Resource..."
kubectl patch app test-app -n test-ns-1 --type=merge -p '{"spec":{"appVersion":"1.0.1","replicas":4}}'
echo -e "${GREEN}✓ Updated Custom Resource${NC}"

echo "Deleting a resource..."
kubectl delete secret test-secret-1 -n test-ns-1
echo -e "${GREEN}✓ Deleted Secret${NC}"

echo "Creating a new resource..."
kubectl create configmap test-config-3 --from-literal=key3=value3 -n test-ns-1
echo -e "${GREEN}✓ Created new ConfigMap${NC}"

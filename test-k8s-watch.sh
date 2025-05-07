#!/bin/bash

# test-k8s-watch.sh
# Helper script for testing the Kubernetes resource watcher
# This script creates a local Kind cluster and deploys various resources
# to test the functionality of the go-k8s-watch tool

set -e

# Colors for better output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Function to print section headers
print_header() {
  echo -e "\n${BLUE}========== $1 ==========${NC}\n"
}

# Function to print success messages
print_success() {
  echo -e "${GREEN}✓ $1${NC}"
}

# Function to print warning messages
print_warning() {
  echo -e "${YELLOW}! $1${NC}"
}

# Function to print error messages
print_error() {
  echo -e "${RED}✗ $1${NC}"
}

# Check if required tools are installed
check_prerequisites() {
  print_header "Checking Prerequisites"

  # Check for kind
  if ! command -v kind &> /dev/null; then
    print_error "kind is not installed. Please install it: https://kind.sigs.k8s.io/docs/user/quick-start/"
    exit 1
  fi
  print_success "kind is installed"

  # Check for kubectl
  if ! command -v kubectl &> /dev/null; then
    print_error "kubectl is not installed. Please install it: https://kubernetes.io/docs/tasks/tools/install-kubectl/"
    exit 1
  fi
  print_success "kubectl is installed"

  # Check for go
  if ! command -v go &> /dev/null; then
    print_error "go is not installed. Please install it: https://golang.org/doc/install"
    exit 1
  fi
  print_success "go is installed"
}

# Create Kind cluster if it doesn't exist
create_cluster() {
  print_header "Setting up Kind Cluster"

  # Check if the cluster already exists
  if kind get clusters | grep -q "k8s-watch-test"; then
    print_warning "Cluster 'k8s-watch-test' already exists. Using the existing cluster."
  else
    echo "Creating Kind cluster 'k8s-watch-test'..."

    # Create a kind config file with multiple nodes
    cat <<EOF > kind-config.yaml
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
name: k8s-watch-test
nodes:
- role: control-plane
- role: worker
- role: worker
EOF

    # Create the cluster
    kind create cluster --config kind-config.yaml

    print_success "Cluster 'k8s-watch-test' created successfully"
  fi

  # Set kubectl context to the kind cluster
  kubectl config use-context kind-k8s-watch-test

  # Wait for the cluster to be ready
  echo "Waiting for the cluster to be ready..."
  kubectl wait --for=condition=Ready nodes --all --timeout=60s

  print_success "Kind cluster is ready"
}

# Create test resources
create_test_resources() {
  print_header "Creating Test Resources"

  # Create namespaces
  echo "Creating test namespaces..."
  kubectl create namespace test-ns-1 --dry-run=client -o yaml | kubectl apply -f -
  kubectl create namespace test-ns-2 --dry-run=client -o yaml | kubectl apply -f -
  print_success "Created test namespaces"

  # Create ConfigMaps
  echo "Creating ConfigMaps..."
  kubectl create configmap test-config-1 --from-literal=key1=value1 -n test-ns-1 --dry-run=client -o yaml | kubectl apply -f -
  kubectl create configmap test-config-2 --from-literal=key2=value2 -n test-ns-2 --dry-run=client -o yaml | kubectl apply -f -
  print_success "Created ConfigMaps"

  # Create Secrets
  echo "Creating Secrets..."
  kubectl create secret generic test-secret-1 --from-literal=password=secret123 -n test-ns-1 --dry-run=client -o yaml | kubectl apply -f -
  print_success "Created Secrets"

  # Create Deployments
  echo "Creating Deployments..."
  cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-deployment
  namespace: test-ns-1
  labels:
    app: nginx
spec:
  replicas: 1
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: nginx:1.20
        ports:
        - containerPort: 80
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: redis-deployment
  namespace: test-ns-2
  labels:
    app: redis
spec:
  replicas: 1
  selector:
    matchLabels:
      app: redis
  template:
    metadata:
      labels:
        app: redis
    spec:
      containers:
      - name: redis
        image: redis:6.0
        ports:
        - containerPort: 6379
EOF
  print_success "Created Deployments"

  # Create Services
  echo "Creating Services..."
  kubectl expose deployment nginx-deployment --port=80 -n test-ns-1 --dry-run=client -o yaml | kubectl apply -f -
  kubectl expose deployment redis-deployment --port=6379 -n test-ns-2 --dry-run=client -o yaml | kubectl apply -f -
  print_success "Created Services"

  # Create a Custom Resource Definition
  echo "Creating a Custom Resource Definition..."
  cat <<EOF | kubectl apply -f -
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: apps.example.com
spec:
  group: example.com
  names:
    kind: App
    listKind: AppList
    plural: apps
    singular: app
  scope: Namespaced
  versions:
  - name: v1
    served: true
    storage: true
    schema:
      openAPIV3Schema:
        type: object
        properties:
          spec:
            type: object
            properties:
              appVersion:
                type: string
              replicas:
                type: integer
EOF
  print_success "Created CRD"

  # Create a Custom Resource
  echo "Creating a Custom Resource..."
  # Wait for CRD to be established
  sleep 5
  cat <<EOF | kubectl apply -f -
apiVersion: example.com/v1
kind: App
metadata:
  name: test-app
  namespace: test-ns-1
spec:
  appVersion: "1.0.0"
  replicas: 3
EOF
  print_success "Created Custom Resource"
}

# Modify resources to trigger watch events
modify_resources() {
  print_header "Modifying Resources to Generate Events"

  # Update a ConfigMap
  echo "Updating ConfigMap..."
  kubectl patch configmap test-config-1 -n test-ns-1 --type=merge -p '{"data":{"key1":"updated-value1","new-key":"new-value"}}'
  print_success "Updated ConfigMap"

  # Scale a deployment
  echo "Scaling Deployment..."
  kubectl scale deployment nginx-deployment -n test-ns-1 --replicas=2
  print_success "Scaled Deployment"

  # Update a Custom Resource
  echo "Updating Custom Resource..."
  kubectl patch app test-app -n test-ns-1 --type=merge -p '{"spec":{"appVersion":"1.0.1","replicas":4}}'
  print_success "Updated Custom Resource"

  # Delete a resource
  echo "Deleting a resource..."
  kubectl delete secret test-secret-1 -n test-ns-1
  print_success "Deleted Secret"

  # Create a new resource
  echo "Creating a new resource..."
  kubectl create configmap test-config-3 --from-literal=key3=value3 -n test-ns-1
  print_success "Created new ConfigMap"
}

# Show command examples for the watcher
show_examples() {
  print_header "Example Commands for the Kubernetes Resource Watcher"

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
}

# Clean up resources
cleanup() {
  print_header "Cleaning up Resources"

  read -p "Do you want to delete the Kind cluster? (y/n): " -n 1 -r
  echo

  if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo "Deleting Kind cluster 'k8s-watch-test'..."
    kind delete cluster --name k8s-watch-test
    print_success "Cluster deleted"
  else
    echo "Keeping the cluster. You can delete it later with:"
    echo "kind delete cluster --name k8s-watch-test"
  fi
}

# Main script
main() {
  check_prerequisites
  create_cluster

  # Pause to allow starting the watcher in another terminal
  print_header "Cluster Ready - Pause Before Creating Resources"
  echo -e "${YELLOW}The Kind cluster is now ready. You can start the resource watcher in another terminal:${NC}"
  echo -e "${GREEN}go run . --namespace=test-ns-1${NC}"
  echo -e "${GREEN}# or to watch everything:${NC}"
  echo -e "${GREEN}go run . --all --all-namespaces${NC}"
  echo
  read -p "Press Enter when you're ready to continue and create test resources..."

  create_test_resources
  show_examples

  # Ask if user wants to modify resources
  read -p "Do you want to modify resources to trigger watch events? (y/n): " -n 1 -r
  echo

  if [[ $REPLY =~ ^[Yy]$ ]]; then
    modify_resources
  fi

  # Ask if user wants to clean up
  read -p "Do you want to clean up now? (y/n): " -n 1 -r
  echo

  if [[ $REPLY =~ ^[Yy]$ ]]; then
    cleanup
  else
    print_warning "You can run this script with 'cleanup' argument later to remove the cluster"
    echo "./test-k8s-watch.sh cleanup"
  fi
}

# Handle script arguments
if [[ "$1" == "cleanup" ]]; then
  cleanup
  exit 0
fi

# Run main function
main
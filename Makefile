# Makefile for go-k8s-watcher
# Replaces functionality from test-k8s-watch.sh script

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

	@if ! command -v kind &> /dev/null; then \
		echo -e "${RED}✗ kind is not installed. Please install it: https://kind.sigs.k8s.io/docs/user/quick-start/${NC}"; \
		exit 1; \
	fi
	@echo -e "${GREEN}✓ kind is installed${NC}"

	@if ! command -v kubectl &> /dev/null; then \
		echo -e "${RED}✗ kubectl is not installed. Please install it: https://kubernetes.io/docs/tasks/tools/install-kubectl/${NC}"; \
		exit 1; \
	fi
	@echo -e "${GREEN}✓ kubectl is installed${NC}"

	@if ! command -v go &> /dev/null; then \
		echo -e "${RED}✗ go is not installed. Please install it: https://golang.org/doc/install${NC}"; \
		exit 1; \
	fi
	@echo -e "${GREEN}✓ go is installed${NC}"

.PHONY: create-cluster
create-cluster: prereqs
	@echo -e "${BLUE}========== Setting up Kind Cluster ==========${NC}\n"

	@if kind get clusters | grep -q "k8s-watch-test"; then \
		echo -e "${YELLOW}! Cluster 'k8s-watch-test' already exists. Using the existing cluster.${NC}"; \
	else \
		echo "Creating Kind cluster 'k8s-watch-test'..."; \
		cat > kind-config.yaml <<EOF \
kind: Cluster\n\
apiVersion: kind.x-k8s.io/v1alpha4\n\
name: k8s-watch-test\n\
nodes:\n\
- role: control-plane\n\
- role: worker\n\
- role: worker\n\
EOF \
		&& kind create cluster --config kind-config.yaml; \
		echo -e "${GREEN}✓ Cluster 'k8s-watch-test' created successfully${NC}"; \
	fi

	@echo "Setting kubectl context to the kind cluster..."
	@kubectl config use-context kind-k8s-watch-test

	@echo "Waiting for the cluster to be ready..."
	@kubectl wait --for=condition=Ready nodes --all --timeout=60s

	@echo -e "${GREEN}✓ Kind cluster is ready${NC}"

.PHONY: resources
resources:
	@echo -e "${BLUE}========== Creating Test Resources ==========${NC}\n"

	@echo "Creating test namespaces..."
	@kubectl create namespace test-ns-1 --dry-run=client -o yaml | kubectl apply -f -
	@kubectl create namespace test-ns-2 --dry-run=client -o yaml | kubectl apply -f -
	@echo -e "${GREEN}✓ Created test namespaces${NC}"

	@echo "Creating ConfigMaps..."
	@kubectl create configmap test-config-1 --from-literal=key1=value1 -n test-ns-1 --dry-run=client -o yaml | kubectl apply -f -
	@kubectl create configmap test-config-2 --from-literal=key2=value2 -n test-ns-2 --dry-run=client -o yaml | kubectl apply -f -
	@echo -e "${GREEN}✓ Created ConfigMaps${NC}"

	@echo "Creating Secrets..."
	@kubectl create secret generic test-secret-1 --from-literal=password=secret123 -n test-ns-1 --dry-run=client -o yaml | kubectl apply -f -
	@echo -e "${GREEN}✓ Created Secrets${NC}"

	@echo "Creating Deployments..."
	@cat <<EOF | kubectl apply -f - \
apiVersion: apps/v1\n\
kind: Deployment\n\
metadata:\n\
  name: nginx-deployment\n\
  namespace: test-ns-1\n\
  labels:\n\
    app: nginx\n\
spec:\n\
  replicas: 1\n\
  selector:\n\
    matchLabels:\n\
      app: nginx\n\
  template:\n\
    metadata:\n\
      labels:\n\
        app: nginx\n\
    spec:\n\
      containers:\n\
      - name: nginx\n\
        image: nginx:1.20\n\
        ports:\n\
        - containerPort: 80\n\
---\n\
apiVersion: apps/v1\n\
kind: Deployment\n\
metadata:\n\
  name: redis-deployment\n\
  namespace: test-ns-2\n\
  labels:\n\
    app: redis\n\
spec:\n\
  replicas: 1\n\
  selector:\n\
    matchLabels:\n\
      app: redis\n\
  template:\n\
    metadata:\n\
      labels:\n\
        app: redis\n\
    spec:\n\
      containers:\n\
      - name: redis\n\
        image: redis:6.0\n\
        ports:\n\
        - containerPort: 6379\n\
EOF
	@echo -e "${GREEN}✓ Created Deployments${NC}"

	@echo "Creating Services..."
	@kubectl expose deployment nginx-deployment --port=80 -n test-ns-1 --dry-run=client -o yaml | kubectl apply -f -
	@kubectl expose deployment redis-deployment --port=6379 -n test-ns-2 --dry-run=client -o yaml | kubectl apply -f -
	@echo -e "${GREEN}✓ Created Services${NC}"

	@echo "Creating a Custom Resource Definition..."
	@cat <<EOF | kubectl apply -f - \
apiVersion: apiextensions.k8s.io/v1\n\
kind: CustomResourceDefinition\n\
metadata:\n\
  name: apps.example.com\n\
spec:\n\
  group: example.com\n\
  names:\n\
    kind: App\n\
    listKind: AppList\n\
    plural: apps\n\
    singular: app\n\
  scope: Namespaced\n\
  versions:\n\
  - name: v1\n\
    served: true\n\
    storage: true\n\
    schema:\n\
      openAPIV3Schema:\n\
        type: object\n\
        properties:\n\
          spec:\n\
            type: object\n\
            properties:\n\
              appVersion:\n\
                type: string\n\
              replicas:\n\
                type: integer\n\
EOF
	@echo -e "${GREEN}✓ Created CRD${NC}"

	@echo "Creating a Custom Resource..."
	@echo "Waiting for CRD to be established..."
	@sleep 5
	@cat <<EOF | kubectl apply -f - \
apiVersion: example.com/v1\n\
kind: App\n\
metadata:\n\
  name: test-app\n\
  namespace: test-ns-1\n\
spec:\n\
  appVersion: "1.0.0"\n\
  replicas: 3\n\
EOF
	@echo -e "${GREEN}✓ Created Custom Resource${NC}"

.PHONY: modify
modify:
	@echo -e "${BLUE}========== Modifying Resources to Generate Events ==========${NC}\n"

	@echo "Updating ConfigMap..."
	@kubectl patch configmap test-config-1 -n test-ns-1 --type=merge -p '{"data":{"key1":"updated-value1","new-key":"new-value"}}'
	@echo -e "${GREEN}✓ Updated ConfigMap${NC}"

	@echo "Scaling Deployment..."
	@kubectl scale deployment nginx-deployment -n test-ns-1 --replicas=2
	@echo -e "${GREEN}✓ Scaled Deployment${NC}"

	@echo "Updating Custom Resource..."
	@kubectl patch app test-app -n test-ns-1 --type=merge -p '{"spec":{"appVersion":"1.0.1","replicas":4}}'
	@echo -e "${GREEN}✓ Updated Custom Resource${NC}"

	@echo "Deleting a resource..."
	@kubectl delete secret test-secret-1 -n test-ns-1
	@echo -e "${GREEN}✓ Deleted Secret${NC}"

	@echo "Creating a new resource..."
	@kubectl create configmap test-config-3 --from-literal=key3=value3 -n test-ns-1
	@echo -e "${GREEN}✓ Created new ConfigMap${NC}"

.PHONY: examples
examples:
	@echo -e "${BLUE}========== Example Commands for the Kubernetes Resource Watcher ==========${NC}\n"

	@echo -e "${GREEN}Watch pods in test-ns-1:${NC}"
	@echo "go run main.go --kind=Pod --api-version=v1 --namespace=test-ns-1"
	@echo

	@echo -e "${GREEN}Watch all resources in test-ns-1:${NC}"
	@echo "go run main.go --all --namespace=test-ns-1"
	@echo

	@echo -e "${GREEN}Watch deployments across all namespaces:${NC}"
	@echo "go run main.go --kind=Deployment --api-version=apps/v1 --all-namespaces"
	@echo

	@echo -e "${GREEN}Watch custom resources:${NC}"
	@echo "go run main.go --kind=App --api-version=example.com/v1 --namespace=test-ns-1"
	@echo

	@echo -e "${GREEN}Watch everything (might be noisy):${NC}"
	@echo "go run main.go --all --all-namespaces"

.PHONY: cleanup
cleanup:
	@echo -e "${BLUE}========== Cleaning up Resources ==========${NC}\n"
	@echo "Deleting Kind cluster 'k8s-watch-test'..."
	@kind delete cluster --name k8s-watch-test
	@echo -e "${GREEN}✓ Cluster deleted${NC}"

# End-to-end test target that runs the full sequence without interactive prompts
.PHONY: e2e-test
e2e-test: create-cluster
	@echo -e "${BLUE}========== Running End-to-End Test ==========${NC}\n"

	@echo -e "${YELLOW}Starting the resource watcher in background...${NC}"
	@# Start the watcher in the background and save its output
	@go run main.go --all --namespace=test-ns-1 > watcher-output.log 2>&1 & \
	WATCHER_PID=$$!; \
	echo "Watcher started with PID: $$WATCHER_PID"; \
	sleep 5; \
	\
	echo -e "${YELLOW}Creating test resources...${NC}"; \
	$(MAKE) resources; \
	sleep 5; \
	\
	echo -e "${YELLOW}Modifying resources to trigger events...${NC}"; \
	$(MAKE) modify; \
	sleep 10; \
	\
	echo -e "${YELLOW}Stopping the watcher...${NC}"; \
	kill $$WATCHER_PID || true; \
	\
	echo -e "${BLUE}========== Test Results ==========${NC}"; \
	echo "Watcher log content:"; \
	cat watcher-output.log | tail -n 20; \
	echo; \
	\
	echo -e "${YELLOW}Do you want to clean up? (Cluster will remain for inspection)${NC}"; \
	echo "Run 'make cleanup' to remove the cluster when done"

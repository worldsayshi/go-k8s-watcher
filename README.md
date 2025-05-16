# go-k8s-watch

A simple proof-of-concept Kubernetes resource watcher that monitors changes to resources in a cluster.

## Features

- Watch any Kubernetes resource type (built-in or custom)
- Monitor specific namespaces or all namespaces
- Detect when resources are added, modified, or deleted
- Automatically reconnect if connection is lost

## Project Structure

- `/manifests` - Kubernetes YAML manifests for testing
- `/scripts` - Helper bash scripts for managing test environment
- `/pkg` - Go packages for the watcher implementation

## Usage

### Run with Makefile

The included Makefile creates a Kind cluster and test resources:

```bash
# Show available commands
make help

# Create a Kind cluster and prepare environment
make create-cluster

# Create test resources
make resources

# Modify resources to generate events
make modify

# Clean up when finished
make cleanup
```

### Run an end-to-end test

Run a complete end-to-end test that creates a cluster, runs the watcher, creates and modifies resources without interaction:

```bash
make e2e-test
```

This will:
1. Create a local Kind cluster if one doesn't exist
2. Start the watcher in the background
3. Create test resources (deployments, services, ConfigMaps, etc.)
4. Modify resources to generate events
5. Display the watcher output
6. Leave the cluster running for inspection (use `make cleanup` when done)

### Run directly

Monitor specific resources:

```bash
go run main.go --kind=Pod --api-version=v1 --namespace=default
```

Monitor all resources across all namespaces:

```bash
go run main.go --all --all-namespaces
```

## Command-line Options

- `--namespace`: Namespace to watch (default: "default")
- `--all-namespaces`: Watch resources across all namespaces
- `--kind`: Specific resource kind to watch (e.g., Pod, Deployment)
- `--api-version`: API version of the resource (e.g., v1, apps/v1)
- `--all`: Watch all available resources
- `--kubeconfig`: Path to kubeconfig file (defaults to $KUBECONFIG or ~/.kube/config)

## Makefile Targets

The Makefile provides the following targets:

- `help`: Show help message with available targets
- `prereqs`: Check for required tools (kind, kubectl, go)
- `create-cluster`: Create a Kind cluster for testing
- `resources`: Create test resources in the cluster
- `modify`: Modify resources to trigger watch events
- `examples`: Show example commands
- `cleanup`: Delete the Kind cluster
- `e2e-test`: Run a complete end-to-end test without user interaction

## Requirements

- Go 1.20+
- kubectl
- kind (for using the test script)
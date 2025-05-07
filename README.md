# go-k8s-watch

A simple proof-of-concept Kubernetes resource watcher that monitors changes to resources in a cluster.

## Features

- Watch any Kubernetes resource type (built-in or custom)
- Monitor specific namespaces or all namespaces
- Detect when resources are added, modified, or deleted
- Automatically reconnect if connection is lost

## Usage

### Run with test script

The included test script creates a Kind cluster and test resources:

```bash
./test-k8s-watch.sh
```

This script will:
1. Create a local Kind cluster if one doesn't exist
2. Pause so you can start the watcher in another terminal
3. Create test resources (deployments, services, ConfigMaps, etc.)
4. Offer to modify resources to generate events
5. Clean up the cluster when finished

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

## Requirements

- Go 1.20+
- kubectl
- kind (for using the test script)
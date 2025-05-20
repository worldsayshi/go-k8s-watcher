# go-k8s-watcher

A Kubernetes resource watcher with both command-line and TUI interfaces that monitors changes to resources in a Kubernetes cluster.

## Makefile Targets

The Makefile provides the following targets:

- `help`: Show help message with available targets
- `prereqs`: Check for required tools (kind, kubectl, go)
- `create-cluster`: Create a Kind cluster for testing
- `resources`: Create test resources in the cluster
- `modify`: Modify resources to trigger watch events
- `examples`: Show example commands
- `cleanup`: Delete the Kind cluster
- `start-watcher`: Start the Kubernetes resource watcher
- `start-tui`: Start the TUI application for resource viewing
- `build`: Build both commands (watcher and TUI)
- `test-only`: Run test sequence (resources & modify) without starting watcher
- `e2e-test`: Run a complete end-to-end test with automatic watcher start/stop## Features

- Watch any Kubernetes resource type (built-in or custom)
- Monitor specific namespaces or all namespaces
- Detect when resources are added, modified, or deleted
- Automatically reconnect if connection is lost
- Interactive TUI interface for searching and viewing resources
- SQLite database for persistent resource storage

## Project Structure

- `/cmd` - Command-line applications
  - `/cmd/watcher` - Command-line watcher tool
  - `/cmd/tui` - Terminal user interface application
- `/manifests` - Kubernetes YAML manifests for testing
- `/pkg` - Go packages
  - `/pkg/watcher` - Kubernetes resource watching implementation
  - `/pkg/db` - SQLite database for resource storage
  - `/pkg/ui` - TUI components using bubbletea
- `/scripts` - Helper bash scripts for managing test environment

## Usage

### Run with Makefile

The included Makefile creates a Kind cluster and test resources:

```bash
# Show available commands
make help

# Create a Kind cluster and prepare environment
make create-cluster

# Start the watcher in a background process
make start-watcher

# Start the TUI application
make start-tui

# Create test resources in a separate terminal
make resources

# Modify resources to generate events
make modify

# Clean up when finished
make cleanup
```

### Command-Line Tools

#### Resource Watcher

```bash
# Build the commands
make build

# Run the watcher directly
./bin/watcher --all --namespace=default
```

#### TUI Application

```bash
# Build the commands
make build

# Run the TUI application
./bin/tui

# With custom kubeconfig
./bin/tui --kubeconfig=/path/to/kubeconfig

# With custom database path
./bin/tui --db=/path/to/database.db
```
make cleanup
```

### Testing Workflows

You can run tests in several ways:

#### Manual Testing (Separate Terminals)

Terminal 1:
```bash
# Create cluster
make create-cluster

# Start the watcher (this will run in the foreground)
make start-watcher
```

Terminal 2:
```bash
# Create and modify resources
make test-only

# Clean up when done
make cleanup
```

#### Automatic End-to-End Test

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
- `start-watcher`: Start the Kubernetes resource watcher in the background
- `test-only`: Run test sequence (resources & modify) without starting watcher
- `e2e-test`: Run a complete end-to-end test with automatic watcher start/stop

## Requirements

- Go 1.20+
- kubectl
- kind (for using the test script)
// Kubernetes Generic Resource Watcher
//
// This tool connects to a Kubernetes cluster and watches for resource changes.
// It can monitor any resource type (built-in or custom) and logs when resources
// are added, modified, or deleted.
//
// Features:
// - Watch any resource type (pods, deployments, services, CRDs, etc.)
// - Monitor specific namespaces or across all namespaces
// - Automatically discover available resources in the cluster
// - Reconnect automatically if connection is lost

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/worldsayshi/go-k8s-watcher/pkg/watcher"
	"k8s.io/apimachinery/pkg/watch"
)

func main() {
	// Parse command line arguments
	namespace := flag.String("namespace", "default", "namespace to watch (for namespaced resources)")
	watchAll := flag.Bool("all", false, "watch all available resources")
	resourceKind := flag.String("kind", "", "specific resource kind to watch (e.g., Pod, Deployment)")
	apiVersion := flag.String("api-version", "", "API version of the resource (e.g., v1, apps/v1)")
	allNamespaces := flag.Bool("all-namespaces", false, "watch resources across all namespaces")
	kubeconfigPath := flag.String("kubeconfig", "", "path to the kubeconfig file")

	flag.Parse()

	// Set up the watcher options
	opts := watcher.Options{
		KubeconfigPath: *kubeconfigPath,
		WatchAll:       *watchAll,
	}

	// Determine namespace to watch
	if *allNamespaces {
		opts.Namespace = "" // Empty string means all namespaces
	} else {
		opts.Namespace = *namespace
	}

	// If specific resource is requested
	if *resourceKind != "" && *apiVersion != "" {
		opts.ResourceTypes = []watcher.ResourceToWatch{
			{
				Kind:       *resourceKind,
				APIVersion: *apiVersion,
				// Let the watcher determine if resource is namespaced
				Namespaced: true, // Default value, will be checked by the watcher
			},
		}
	}

	// Create a new watcher
	k8sWatcher, err := watcher.NewWatcher(opts)
	if err != nil {
		log.Fatalf("Failed to create watcher: %v", err)
	}

	// Create a context that can be canceled on SIGINT/SIGTERM
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle termination signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("Received termination signal, shutting down...")
		cancel()
	}()

	// Log which namespace we're watching
	if *allNamespaces {
		fmt.Println("Starting to watch resources across all namespaces")
	} else {
		fmt.Printf("Starting to watch resources in namespace: %s\n", *namespace)
	}

	// Start the watcher with our event handler
	if err := k8sWatcher.Start(ctx, eventHandler); err != nil {
		log.Fatalf("Failed to start watcher: %v", err)
	}

	fmt.Println("Watchers started. Press Ctrl+C to exit.")

	// Wait for context to be done (from signal handler)
	<-ctx.Done()

	// Stop the watcher gracefully
	k8sWatcher.Stop()
	fmt.Println("Watcher stopped cleanly")
}

// eventHandler processes resource events
func eventHandler(event watcher.ResourceEvent) {
	var logMsg string

	// Create a resource string for display
	resourceStr := event.Resource.Kind
	if group, version := watcher.SplitAPIVersion(event.Resource.APIVersion); group != "" {
		resourceStr = fmt.Sprintf("%s.%s/%s", resourceStr, group, version)
	} else {
		resourceStr = fmt.Sprintf("%s/%s", resourceStr, version)
	}

	// Format based on event type
	switch event.Type {
	case watch.Added:
		logMsg = fmt.Sprintf("[ADDED] %s: %s, Namespace: %s, ResourceVersion: %s",
			resourceStr, event.Name, event.Namespace, event.ResourceVersion)

		// Add condensed spec info if available
		if spec, found := getSpecFromObject(event.Object); found && len(spec) > 0 {
			if len(spec) > 200 {
				spec = spec[:200] + "... (truncated)"
			}
			logMsg += fmt.Sprintf(", Spec: %s", spec)
		}

	case watch.Modified:
		if event.PreviousResourceVersion == event.ResourceVersion {
			logMsg = fmt.Sprintf("[MODIFIED-NO-CHANGE] %s: %s, Namespace: %s, ResourceVersion unchanged: %s",
				resourceStr, event.Name, event.Namespace, event.ResourceVersion)
		} else {
			logMsg = fmt.Sprintf("[MODIFIED] %s: %s, Namespace: %s, ResourceVersion: %s -> %s",
				resourceStr, event.Name, event.Namespace, event.PreviousResourceVersion, event.ResourceVersion)

			// Add condensed spec info if available
			if spec, found := getSpecFromObject(event.Object); found && len(spec) > 0 {
				if len(spec) > 200 {
					spec = spec[:200] + "... (truncated)"
				}
				logMsg += fmt.Sprintf(", Spec: %s", spec)
			}
		}

	case watch.Deleted:
		logMsg = fmt.Sprintf("[DELETED] %s: %s, Namespace: %s, Final ResourceVersion: %s",
			resourceStr, event.Name, event.Namespace, event.ResourceVersion)

	case watch.Error:
		if event.Error != nil {
			logMsg = fmt.Sprintf("[ERROR] %s: %s, Namespace: %s, Error: %v",
				resourceStr, event.Name, event.Namespace, event.Error)
		} else {
			logMsg = fmt.Sprintf("[ERROR] %s: %s, Namespace: %s, Unknown error",
				resourceStr, event.Name, event.Namespace)
		}
	}

	// Log the event
	log.Println(logMsg)

	// Debug extra information for specific objects we're interested in
	if event.Type == watch.Modified && (contains(event.Name, "nginx") ||
		contains(event.Name, "test-app") ||
		contains(event.Name, "test-config")) {
		log.Printf("[DEBUG] Detected change to watched object: %s/%s", event.Namespace, event.Name)
	}
}

// getSpecFromObject extracts and formats the spec section from an object
func getSpecFromObject(obj map[string]interface{}) (string, bool) {
	spec, found := obj["spec"]
	if !found {
		return "", false
	}

	specBytes, err := json.Marshal(spec)
	if err != nil {
		return "", false
	}

	return string(specBytes), true
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

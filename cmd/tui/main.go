// Resource TUI for Kubernetes
//
// This command-line tool connects to a Kubernetes cluster, watches resources,
// stores them in SQLite, and provides a TUI to search and display resources.

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/worldsayshi/go-k8s-watcher/pkg/db"
	"github.com/worldsayshi/go-k8s-watcher/pkg/ui"
	"github.com/worldsayshi/go-k8s-watcher/pkg/watcher"
	"k8s.io/apimachinery/pkg/watch"
)

func main() {
	// Parse command-line flags
	kubeconfigPath := flag.String("kubeconfig", "", "path to the kubeconfig file")
	dbPath := flag.String("db", filepath.Join(os.TempDir(), "k8s-resources.db"), "path to the SQLite database file")
	logFilePath := flag.String("log", filepath.Join(os.TempDir(), "k8s-tui.log"), "path to the log file")
	flag.Parse()

	// Set up logging to a file instead of stdout
	logFile, err := os.OpenFile(*logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open log file: %v\n", err)
		os.Exit(1)
	}
	defer logFile.Close()
	log.SetOutput(logFile)
	log.Printf("TUI application started, logs redirected to %s", *logFilePath)

	// Create database store
	store, err := db.New(*dbPath)
	if err != nil {
		log.Fatalf("Failed to create database: %v", err)
	}
	defer store.Close()

	// Setup watcher options - watch all resources in all namespaces
	opts := watcher.Options{
		KubeconfigPath: *kubeconfigPath,
		WatchAll:       true,
		Namespace:      "", // Empty string means all namespaces
	}

	// Create Kubernetes watcher
	k8sWatcher, err := watcher.NewWatcher(opts)
	if err != nil {
		log.Fatalf("Failed to create watcher: %v", err)
	}

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())

	// Setup signal handling for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("Received termination signal, shutting down...")
		cancel()
	}()

	// Start the watcher in a separate goroutine
	go func() {
		// Event handler that stores resources in the database
		eventHandler := func(event watcher.ResourceEvent) {
			resourceData, _ := json.Marshal(event.Object)

			switch event.Type {
			case watch.Added, watch.Modified:
				// Add or update resource in the database
				r := db.Resource{
					Name:            event.Name,
					Namespace:       event.Namespace,
					Kind:            event.Resource.Kind,
					APIVersion:      event.Resource.APIVersion,
					ResourceVersion: event.ResourceVersion,
					Data:            string(resourceData),
				}
				if err := store.Upsert(r); err != nil {
					log.Printf("Failed to store resource: %v", err)
				}

			case watch.Deleted:
				// Remove resource from the database
				if err := store.Delete(
					event.Resource.Kind,
					event.Resource.APIVersion,
					event.Namespace,
					event.Name,
				); err != nil {
					log.Printf("Failed to delete resource: %v", err)
				}
			}
		}

		if err := k8sWatcher.Start(ctx, eventHandler); err != nil {
			log.Printf("Failed to start watcher: %v", err)
			cancel()
			return
		}

		log.Println("Resource watcher started. Collecting resources...")
	}()

	// Run the TUI
	if err := ui.Run(store); err != nil {
		log.Printf("Error in UI: %v", err)
	}

	// Clean up when UI exits
	cancel()
	k8sWatcher.Stop()
}

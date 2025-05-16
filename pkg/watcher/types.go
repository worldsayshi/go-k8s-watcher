// Package watcher provides functionality for watching Kubernetes resources.
package watcher

import (
	"context"

	"k8s.io/apimachinery/pkg/watch"
)

// ResourceToWatch represents a Kubernetes resource to watch
type ResourceToWatch struct {
	Kind       string
	APIVersion string
	Namespaced bool
}

// ResourceEvent represents an event that occurred on a Kubernetes resource
type ResourceEvent struct {
	// Type of event (Added, Modified, Deleted, Error)
	Type watch.EventType
	// Resource is the resource type information
	Resource ResourceToWatch
	// Name of the resource
	Name string
	// Namespace of the resource (empty for cluster-scoped resources)
	Namespace string
	// ResourceVersion of the object involved in the event
	ResourceVersion string
	// PreviousResourceVersion if this is a modification event
	PreviousResourceVersion string
	// Object is the raw object data
	Object map[string]interface{}
	// Error information if the event type is Error
	Error error
}

// EventHandler is a callback function that is invoked when resource events occur
type EventHandler func(event ResourceEvent)

// Options configures the behavior of the watcher
type Options struct {
	// Namespace to watch (empty string for all namespaces)
	Namespace string
	// ResourceTypes to watch (empty for default set)
	ResourceTypes []ResourceToWatch
	// WatchAll resources discovered in the API
	WatchAll bool
	// KubeconfigPath explicitly sets a kubeconfig file path
	KubeconfigPath string
}

// ResourceWatcher defines the interface for watching Kubernetes resources
type ResourceWatcher interface {
	// Start begins watching resources and calls the handler for events
	// Returns an error if the watcher could not be started
	Start(ctx context.Context, handler EventHandler) error

	// Stop halts all watchers
	Stop()

	// IsWatching returns true if the watcher is currently active
	IsWatching() bool
}

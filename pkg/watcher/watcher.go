package watcher

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/utils/ptr"
)

// K8sWatcher implements ResourceWatcher
type K8sWatcher struct {
	options        Options
	dynamicClient  dynamic.Interface
	discovery      *discovery.DiscoveryClient
	restMapper     *restmapper.DeferredDiscoveryRESTMapper
	activeWatchers sync.WaitGroup
	stopCh         chan struct{}
	watching       bool
	mu             sync.RWMutex
}

// DefaultResourceTypes returns a set of common resource types to watch
func DefaultResourceTypes() []ResourceToWatch {
	return []ResourceToWatch{
		{Kind: "Pod", APIVersion: "v1", Namespaced: true},
		{Kind: "Deployment", APIVersion: "apps/v1", Namespaced: true},
		{Kind: "Service", APIVersion: "v1", Namespaced: true},
		{Kind: "ConfigMap", APIVersion: "v1", Namespaced: true},
		{Kind: "Namespace", APIVersion: "v1", Namespaced: false},
	}
}

// NewWatcher creates a new Kubernetes resource watcher
func NewWatcher(options Options) (*K8sWatcher, error) {
	// Build Kubernetes client configuration
	configLoadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if options.KubeconfigPath != "" {
		configLoadingRules.ExplicitPath = options.KubeconfigPath
	}

	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		configLoadingRules,
		&clientcmd.ConfigOverrides{})

	// Get REST config
	config, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("error building kubeconfig: %v", err)
	}

	// Create dynamic client
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("error creating dynamic client: %v", err)
	}

	// Create discovery client
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("error creating discovery client: %v", err)
	}

	// Create REST mapper for resource discovery
	cachedDiscoveryClient := memory.NewMemCacheClient(discoveryClient)
	restMapper := restmapper.NewDeferredDiscoveryRESTMapper(cachedDiscoveryClient)

	// Set defaults if not specified
	if len(options.ResourceTypes) == 0 && !options.WatchAll {
		options.ResourceTypes = DefaultResourceTypes()
	}

	return &K8sWatcher{
		options:       options,
		dynamicClient: dynamicClient,
		discovery:     discoveryClient,
		restMapper:    restMapper,
		stopCh:        make(chan struct{}),
	}, nil
}

// Start begins watching resources
func (w *K8sWatcher) Start(ctx context.Context, handler EventHandler) error {
	w.mu.Lock()
	if w.watching {
		w.mu.Unlock()
		return fmt.Errorf("watcher is already running")
	}
	w.watching = true
	w.stopCh = make(chan struct{})
	w.mu.Unlock()

	// Context that can be canceled to stop all watchers
	watchCtx, cancel := context.WithCancel(ctx)
	go func() {
		select {
		case <-watchCtx.Done():
			// Context was canceled externally
		case <-w.stopCh:
			// Stop was called
			cancel()
		}
	}()

	var resourcesToWatch []ResourceToWatch

	if w.options.WatchAll {
		var err error
		resourcesToWatch, err = w.discoverAllResources()
		if err != nil {
			return fmt.Errorf("error discovering resources: %v", err)
		}
	} else {
		resourcesToWatch = w.options.ResourceTypes
	}

	log.Printf("Starting to watch %d resource types", len(resourcesToWatch))

	// Start watchers for all resource types
	for _, resource := range resourcesToWatch {
		w.startResourceWatcher(watchCtx, resource, w.options.Namespace, handler)
	}

	return nil
}

// Stop halts all watchers
func (w *K8sWatcher) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.watching {
		return
	}

	close(w.stopCh)
	w.watching = false

	// Wait for all watchers to finish (with a timeout)
	done := make(chan struct{})
	go func() {
		w.activeWatchers.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All watchers finished cleanly
	case <-time.After(10 * time.Second):
		// Timeout reached, some watchers might still be running
		log.Printf("Timed out waiting for all watchers to stop")
	}
}

// IsWatching returns true if the watcher is currently active
func (w *K8sWatcher) IsWatching() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.watching
}

// discoverAllResources finds all watchable resources in the cluster
func (w *K8sWatcher) discoverAllResources() ([]ResourceToWatch, error) {
	var resources []ResourceToWatch
	processedResources := make(map[string]bool)

	// Get all API resources
	_, resourceLists, err := w.discovery.ServerGroupsAndResources()
	if err != nil {
		// This error is expected since some resources might not be discoverable
		if !discovery.IsGroupDiscoveryFailedError(err) {
			return nil, err
		}
		log.Printf("Warning: Some API groups couldn't be discovered: %v", err)
	}

	// Process resource lists
	for _, resList := range resourceLists {
		for _, r := range resList.APIResources {
			// Skip resources we can't watch or are subresources
			if !contains(r.Verbs, "watch") || strings.Contains(r.Name, "/") {
				continue
			}

			// Create a unique key for this resource to avoid duplicates
			resourceKey := fmt.Sprintf("%s/%s/%s", resList.GroupVersion, r.Kind, r.Name)
			if processedResources[resourceKey] {
				continue
			}

			processedResources[resourceKey] = true

			group, version := splitAPIVersion(resList.GroupVersion)
			apiVersion := resList.GroupVersion
			if group == "" {
				apiVersion = version // core API has no group prefix
			}

			resources = append(resources, ResourceToWatch{
				Kind:       r.Kind,
				APIVersion: apiVersion,
				Namespaced: r.Namespaced,
			})
		}
	}

	return resources, nil
}

// startResourceWatcher begins watching a specific resource type
func (w *K8sWatcher) startResourceWatcher(
	ctx context.Context,
	resource ResourceToWatch,
	namespace string,
	handler EventHandler,
) {
	group, version := splitAPIVersion(resource.APIVersion)

	// Create GroupVersionResource
	gvr := schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: getResourceNameFromKind(resource.Kind),
	}

	// Determine if we should watch a specific namespace
	var resourceInterface dynamic.ResourceInterface
	if resource.Namespaced && namespace != "" {
		resourceInterface = w.dynamicClient.Resource(gvr).Namespace(namespace)
	} else {
		resourceInterface = w.dynamicClient.Resource(gvr)
	}

	resourceStr := resource.Kind
	if group != "" {
		resourceStr = fmt.Sprintf("%s.%s/%s", resourceStr, group, version)
	} else {
		resourceStr = fmt.Sprintf("%s/%s", resourceStr, version)
	}

	log.Printf("Starting watcher for: %s", resourceStr)

	// Increment active watcher counter
	w.activeWatchers.Add(1)

	go func() {
		defer w.activeWatchers.Done()

		// Track resource versions for detecting real changes
		resourceVersions := make(map[string]string)
		retries := 0

		for {
			// Check if context is done
			select {
			case <-ctx.Done():
				log.Printf("Stopping watcher for %s (context canceled)", resourceStr)
				return
			default:
				// Continue
			}

			// Create watcher with timeout to ensure connection doesn't hang
			watchContext, watchCancel := context.WithTimeout(ctx, 60*time.Minute)

			watcher, err := resourceInterface.Watch(watchContext, metav1.ListOptions{
				TimeoutSeconds: ptr.To(int64(3600)), // 1 hour server-side timeout
			})

			if err != nil {
				if retries > 5 {
					log.Printf("Giving up on watching %s after multiple failures: %v", resourceStr, err)
					watchCancel()
					return
				}

				if strings.Contains(err.Error(), "could not find the requested resource") {
					log.Printf("Resource %s isn't available in this cluster, skipping", resourceStr)
					watchCancel()
					return
				}

				log.Printf("Error watching %s: %v (will retry)", resourceStr, err)
				watchCancel()
				retries++
				time.Sleep(time.Duration(2*retries) * time.Second) // Exponential backoff
				continue
			}

			retries = 0 // Reset retries on successful watch

			log.Printf("Watcher started for %s", resourceStr)
			ch := watcher.ResultChan()

			for {
				select {
				case <-ctx.Done():
					watcher.Stop()
					watchCancel()
					log.Printf("Stopping watcher for %s (context canceled)", resourceStr)
					return

				case event, ok := <-ch:
					if !ok {
						watchCancel()
						log.Printf("Watch channel closed for %s, restarting...", resourceStr)
						time.Sleep(1 * time.Second)
						break
					}

					w.handleEvent(event, resource, resourceVersions, handler, resourceStr)
				}
			}
		}
	}()
}

// handleEvent processes an event from the watch channel
func (w *K8sWatcher) handleEvent(
	event watch.Event,
	resource ResourceToWatch,
	resourceVersions map[string]string,
	handler EventHandler,
	resourceStr string,
) {
	obj, ok := event.Object.(*unstructured.Unstructured)
	if !ok {
		log.Printf("Unexpected object type: %T", event.Object)
		return
	}

	// Extract metadata
	name, _, _ := unstructured.NestedString(obj.Object, "metadata", "name")
	namespace, _, _ := unstructured.NestedString(obj.Object, "metadata", "namespace")
	resourceVersion, _, _ := unstructured.NestedString(obj.Object, "metadata", "resourceVersion")

	// Create a key for this resource
	resourceKey := fmt.Sprintf("%s/%s", namespace, name)

	// Create and populate the event
	resourceEvent := ResourceEvent{
		Type:            event.Type,
		Resource:        resource,
		Name:            name,
		Namespace:       namespace,
		ResourceVersion: resourceVersion,
		Object:          obj.Object,
	}

	switch event.Type {
	case watch.Added:
		resourceVersions[resourceKey] = resourceVersion

	case watch.Modified:
		oldRV := resourceVersions[resourceKey]
		resourceEvent.PreviousResourceVersion = oldRV
		resourceVersions[resourceKey] = resourceVersion

	case watch.Deleted:
		delete(resourceVersions, resourceKey)

	case watch.Error:
		status, ok := event.Object.(*metav1.Status)
		if ok {
			resourceEvent.Error = fmt.Errorf("error event: %s", status.Message)
		} else {
			resourceEvent.Error = fmt.Errorf("unknown error event")
		}
	}

	// Call the handler with the event
	handler(resourceEvent)
}

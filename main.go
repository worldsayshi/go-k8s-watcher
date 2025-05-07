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
//
// Usage:
//   go run main.go                                        # Watch default resources in default namespace
//   go run main.go --all --all-namespaces                 # Watch all resources across all namespaces
//   go run main.go --kind=Pod --api-version=v1            # Watch only pods
//   go run main.go --kind=Deployment --api-version=apps/v1 --namespace=kube-system # Watch deployments in kube-system

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"path/filepath"
	"strings"
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
	"k8s.io/client-go/util/homedir"
)

// ResourceToWatch represents a Kubernetes resource to watch
type ResourceToWatch struct {
	Kind       string
	APIVersion string
	Namespaced bool
}

func main() {
	// Set up Kubernetes client configuration
	var kubeconfig *string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}

	namespace := flag.String("namespace", "default", "namespace to watch (for namespaced resources)")
	watchAll := flag.Bool("all", false, "watch all available resources")
	resourceKind := flag.String("kind", "", "specific resource kind to watch (e.g., Pod, Deployment)")
	apiVersion := flag.String("api-version", "", "API version of the resource (e.g., v1, apps/v1)")
	allNamespaces := flag.Bool("all-namespaces", false, "watch resources across all namespaces")

	flag.Parse()

	// Build the config from the path
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		log.Fatalf("Error building kubeconfig: %v", err)
	}

	// Create dynamic client
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		log.Fatalf("Error creating dynamic client: %v", err)
	}

	// Create discovery client for resource information
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		log.Fatalf("Error creating discovery client: %v", err)
	}

	// Create a RESTMapper to map resources to their API paths
	cachedDiscoveryClient := memory.NewMemCacheClient(discoveryClient)
	restMapper := restmapper.NewDeferredDiscoveryRESTMapper(cachedDiscoveryClient)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Choose what resources to watch
	var resourcesToWatch []ResourceToWatch

	if *watchAll {
		// Discover all resources in the cluster
		fmt.Println("Discovering all available resources...")
		apiGroups, _, err := discoveryClient.ServerGroupsAndResources()
		if err != nil {
			log.Fatalf("Error getting server resources: %v", err)
		}

		for _, apiGroup := range apiGroups {
			groupVersion := apiGroup.PreferredVersion.GroupVersion
			resources, err := discoveryClient.ServerResourcesForGroupVersion(groupVersion)
			if err != nil {
				log.Printf("Error getting resources for group version %s: %v", groupVersion, err)
				continue
			}

			for _, r := range resources.APIResources {
				// Skip subresources like pods/log or deployments/scale
				if len(r.Group) == 0 && r.Version == "" {
					r.Group = resources.GroupVersion
					if !contains(r.Verbs, "watch") || strings.Contains(r.Name, "/") {
						continue
					}
				}

				parts := splitAPIVersion(resources.GroupVersion)
				group, version := parts[0], parts[1]
				apiVersion := resources.GroupVersion
				if group == "" {
					apiVersion = version // core API has no group prefix
				}

				resourcesToWatch = append(resourcesToWatch, ResourceToWatch{
					Kind:       r.Kind,
					APIVersion: apiVersion,
					Namespaced: r.Namespaced,
				})
			}
		}
	} else if *resourceKind != "" && *apiVersion != "" {
		// Watch specific resource type
		namespaced := true // Default to namespaced resources

		// Try to determine if the resource is namespaced
		if *apiVersion != "" && *resourceKind != "" {
			parts := splitAPIVersion(*apiVersion)
			group, version := parts[0], parts[1]

			gv := schema.GroupVersion{Group: group, Version: version}
			resources, err := discoveryClient.ServerResourcesForGroupVersion(gv.String())
			if err == nil {
				for _, r := range resources.APIResources {
					if r.Kind == *resourceKind {
						namespaced = r.Namespaced
						break
					}
				}
			}
		}

		resourcesToWatch = append(resourcesToWatch, ResourceToWatch{
			Kind:       *resourceKind,
			APIVersion: *apiVersion,
			Namespaced: namespaced,
		})
	} else {
		// Default to some common resources
		resourcesToWatch = []ResourceToWatch{
			{Kind: "Pod", APIVersion: "v1", Namespaced: true},
			{Kind: "Deployment", APIVersion: "apps/v1", Namespaced: true},
			{Kind: "Service", APIVersion: "v1", Namespaced: true},
			{Kind: "ConfigMap", APIVersion: "v1", Namespaced: true},
			{Kind: "Namespace", APIVersion: "v1", Namespaced: false},
		}
	}

	// Determine which namespace(s) to watch
	watchNamespace := *namespace
	if *allNamespaces {
		watchNamespace = "" // Empty string means all namespaces
	}

	// Start watching each resource
	fmt.Printf("Starting to watch resources in namespace: %s\n", watchNamespace)
	if watchNamespace == "" {
		fmt.Println("Watching across all namespaces")
	}

	for _, resource := range resourcesToWatch {
		watchResourceType(ctx, dynamicClient, restMapper, resource, watchNamespace)
	}

	fmt.Println("Watchers started. Press Ctrl+C to exit.")
	select {} // Keep the program running
}

func watchResourceType(ctx context.Context, client dynamic.Interface, mapper *restmapper.DeferredDiscoveryRESTMapper, resource ResourceToWatch, namespace string) {
	parts := splitAPIVersion(resource.APIVersion)
	group, version := parts[0], parts[1]

	// Create a new GVR (GroupVersionResource)
	gvr := schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: getResourceNameFromKind(resource.Kind),
	}

	// Determine if we should watch a specific namespace or all namespaces
	var resourceInterface dynamic.ResourceInterface
	if resource.Namespaced && namespace != "" {
		resourceInterface = client.Resource(gvr).Namespace(namespace)
	} else {
		resourceInterface = client.Resource(gvr)
	}

	watchStr := resource.Kind
	if group != "" {
		watchStr = fmt.Sprintf("%s.%s/%s", watchStr, group, version)
	} else {
		watchStr = fmt.Sprintf("%s/%s", watchStr, version)
	}

	fmt.Printf("Starting watcher for: %s\n", watchStr)

	go func() {
		for {
			watcher, err := resourceInterface.Watch(ctx, metav1.ListOptions{})
			if err != nil {
				log.Printf("Error watching %s: %v", watchStr, err)
				time.Sleep(5 * time.Second)
				continue
			}

			ch := watcher.ResultChan()
			log.Printf("Watcher started for %s", watchStr)

			for event := range ch {
				obj, ok := event.Object.(*unstructured.Unstructured)
				if !ok {
					log.Printf("Unexpected object type: %T", event.Object)
					continue
				}

				// Extract metadata
				objName, _, _ := unstructured.NestedString(obj.Object, "metadata", "name")
				objNamespace, _, _ := unstructured.NestedString(obj.Object, "metadata", "namespace")
				resourceVersion, _, _ := unstructured.NestedString(obj.Object, "metadata", "resourceVersion")

				// Output based on event type
				switch event.Type {
				case watch.Added:
					log.Printf("[ADDED] %s: %s, Namespace: %s, ResourceVersion: %s",
						watchStr, objName, objNamespace, resourceVersion)
				case watch.Modified:
					log.Printf("[MODIFIED] %s: %s, Namespace: %s, ResourceVersion: %s",
						watchStr, objName, objNamespace, resourceVersion)
				case watch.Deleted:
					log.Printf("[DELETED] %s: %s, Namespace: %s",
						watchStr, objName, objNamespace)
				case watch.Error:
					log.Printf("[ERROR] %s: %s, Namespace: %s",
						watchStr, objName, objNamespace)
				}
			}

			log.Printf("Watcher channel closed for %s, restarting...", watchStr)
			time.Sleep(1 * time.Second)
		}
	}()
}

// Helper function to pluralize common Kubernetes resource kinds
func getResourceNameFromKind(kind string) string {
	kindToResource := map[string]string{
		"Pod":                      "pods",
		"Deployment":               "deployments",
		"Service":                  "services",
		"ConfigMap":                "configmaps",
		"Secret":                   "secrets",
		"Namespace":                "namespaces",
		"Node":                     "nodes",
		"PersistentVolume":         "persistentvolumes",
		"PersistentVolumeClaim":    "persistentvolumeclaims",
		"Ingress":                  "ingresses",
		"Job":                      "jobs",
		"CronJob":                  "cronjobs",
		"StatefulSet":              "statefulsets",
		"DaemonSet":                "daemonsets",
		"ServiceAccount":           "serviceaccounts",
		"Role":                     "roles",
		"RoleBinding":              "rolebindings",
		"ClusterRole":              "clusterroles",
		"ClusterRoleBinding":       "clusterrolebindings",
		"CustomResourceDefinition": "customresourcedefinitions",
	}

	if resource, ok := kindToResource[kind]; ok {
		return resource
	}

	// For unknown kinds, attempt to make a reasonable guess
	// Default to lowercase + append "s" for English pluralization
	return fmt.Sprintf("%ss", toLowerCamelCase(kind))
}

// Helper function to split API version into group and version
func splitAPIVersion(apiVersion string) []string {
	parts := []string{"", ""}
	if apiVersion == "v1" {
		// Special case for core API group
		parts[1] = apiVersion
	} else if idx := splitBySlash(apiVersion); idx != -1 {
		parts[0] = apiVersion[:idx]
		parts[1] = apiVersion[idx+1:]
	} else {
		parts[1] = apiVersion
	}
	return parts
}

// Helper function to find the index of '/' in a string
func splitBySlash(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] == '/' {
			return i
		}
	}
	return -1
}

// Helper function to check if a string slice contains a value
func contains(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

// Helper function to convert a string to lowerCamelCase
func toLowerCamelCase(s string) string {
	if len(s) == 0 {
		return s
	}
	return string(s[0]) + s[1:]
}

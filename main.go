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
	"net"
	"os/exec"
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
	// Set up command line flags
	var kubeconfigFlag *string
	if home := homedir.HomeDir(); home != "" {
		// Use $KUBECONFIG environment variable if set, otherwise default to ~/.kube/config
		kubeconfigFlag = flag.String("kubeconfig", "", "path to the kubeconfig file (overrides $KUBECONFIG and default)")
	} else {
		kubeconfigFlag = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}

	namespace := flag.String("namespace", "default", "namespace to watch (for namespaced resources)")
	watchAll := flag.Bool("all", false, "watch all available resources")
	resourceKind := flag.String("kind", "", "specific resource kind to watch (e.g., Pod, Deployment)")
	apiVersion := flag.String("api-version", "", "API version of the resource (e.g., v1, apps/v1)")
	allNamespaces := flag.Bool("all-namespaces", false, "watch resources across all namespaces")
	// useInClusterConfig := flag.Bool("in-cluster", false, "use in-cluster config when running inside a pod")

	flag.Parse()

	// Build configuration from kubeconfig
	fmt.Println("Loading Kubernetes configuration...")

	// Handle kubeconfig path priority:
	// 1. --kubeconfig flag (highest priority)
	// 2. KUBECONFIG environment variable
	// 3. Default ~/.kube/config path
	configLoadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if *kubeconfigFlag != "" {
		// If explicit flag is provided, use only that
		configLoadingRules.ExplicitPath = *kubeconfigFlag
	}

	// The loading rules will automatically read from $KUBECONFIG if set
	// or fall back to ~/.kube/config if not specified

	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		configLoadingRules,
		&clientcmd.ConfigOverrides{})

	// Get the resulting kubeconfig
	config, err := clientConfig.ClientConfig()
	if err != nil {
		log.Fatalf("Error building kubeconfig: %v", err)
	}

	// Log which kubeconfig is being used
	rawConfig, err := clientConfig.RawConfig()
	if err == nil {
		currentContext := rawConfig.CurrentContext
		if currentCtx, ok := rawConfig.Contexts[currentContext]; ok {
			log.Printf("Using kubeconfig context: %s (cluster: %s)", currentContext, currentCtx.Cluster)
		}
	}

	log.Printf("Connecting to Kubernetes API server at: %s", config.Host)

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

	// Verify connection to API server with timeout
	verifyCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		_, err = discoveryClient.ServerVersion()
		if err != nil {
			log.Printf("Warning: Could not connect to Kubernetes API server: %v", err)
			log.Printf("Ensure your Kubernetes cluster is running and accessible.")
		} else {
			log.Printf("Successfully connected to Kubernetes API server at %s", config.Host)
		}
		cancel()
	}()

	<-verifyCtx.Done()
	if verifyCtx.Err() == context.DeadlineExceeded {
		log.Printf("Warning: Kubernetes API server connection timed out. Continuing anyway...")
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

// Helper function to get the host IP address for WSL
func getHostIP() string {
	// For WSL, we need to get the Windows host IP
	// First try to get the IP from /etc/resolv.conf (WSL2)
	cmd := exec.Command("bash", "-c", "grep nameserver /etc/resolv.conf | cut -d' ' -f2 | head -n 1")
	out, err := cmd.Output()
	if err == nil && len(out) > 0 {
		ip := strings.TrimSpace(string(out))
		if ip != "127.0.0.1" && ip != "::1" {
			log.Printf("Found WSL2 host IP from resolv.conf: %s", ip)
			return ip
		}
	}

	// Try to get the host.docker.internal via DNS lookup
	ips, err := net.LookupHost("host.docker.internal")
	if err == nil && len(ips) > 0 {
		log.Printf("Found host.docker.internal IP: %s", ips[0])
		return ips[0]
	}

	// Try the routing table approach (alternative for WSL1)
	cmd = exec.Command("bash", "-c", "ip route show | grep default | awk '{print $3}'")
	out, err = cmd.Output()
	if err == nil && len(out) > 0 {
		ip := strings.TrimSpace(string(out))
		if ip != "" {
			log.Printf("Found default gateway IP from routing table: %s", ip)
			return ip
		}
	}

	// As a last resort, try a common Docker Desktop port forwarding IP
	log.Printf("Could not determine Windows host IP from WSL. Using default Docker Desktop IP 127.0.0.1")
	return "127.0.0.1"
}

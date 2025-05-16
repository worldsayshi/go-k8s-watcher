package watcher

import (
	"fmt"
	"strings"
)

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
func SplitAPIVersion(apiVersion string) (string, string) {
	if apiVersion == "v1" {
		// Special case for core API group
		return "", apiVersion
	}

	if idx := strings.Index(apiVersion, "/"); idx != -1 {
		return apiVersion[:idx], apiVersion[idx+1:]
	}

	return "", apiVersion
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
	// Convert first character to lowercase
	return strings.ToLower(string(s[0])) + s[1:]
}

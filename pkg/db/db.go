// Package db provides SQLite storage for Kubernetes resources
package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"

	_ "github.com/mattn/go-sqlite3"
)

// ResourceStore manages the SQLite database for Kubernetes resources
type ResourceStore struct {
	db   *sql.DB
	mu   sync.RWMutex
	path string
}

// Resource represents a Kubernetes resource in the database
type Resource struct {
	ID              int64  `json:"-"`
	Name            string `json:"name"`
	Namespace       string `json:"namespace"`
	Kind            string `json:"kind"`
	APIVersion      string `json:"apiVersion"`
	ResourceVersion string `json:"resourceVersion"`
	Data            string `json:"data"`
}

// New creates a new ResourceStore with the specified database file
func New(dbPath string) (*ResourceStore, error) {
	// Ensure the directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory for database: %v", err)
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}

	store := &ResourceStore{
		db:   db,
		path: dbPath,
	}

	if err := store.initialize(); err != nil {
		db.Close()
		return nil, err
	}

	return store, nil
}

// Initialize sets up the database schema
func (s *ResourceStore) initialize() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Create resources table if it doesn't exist
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS resources (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			namespace TEXT NOT NULL,
			kind TEXT NOT NULL,
			api_version TEXT NOT NULL,
			resource_version TEXT NOT NULL,
			data TEXT NOT NULL,
			UNIQUE(kind, api_version, namespace, name)
		);
		CREATE INDEX IF NOT EXISTS idx_resources_search ON resources(name, namespace, kind);
	`)
	if err != nil {
		return fmt.Errorf("failed to create tables: %v", err)
	}

	return nil
}

// Close closes the database connection
func (s *ResourceStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.db.Close()
}

// Upsert adds or updates a resource in the database
func (s *ResourceStore) Upsert(resource Resource) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`
		INSERT INTO resources (name, namespace, kind, api_version, resource_version, data)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(kind, api_version, namespace, name)
		DO UPDATE SET resource_version = ?, data = ?
	`, resource.Name, resource.Namespace, resource.Kind, resource.APIVersion,
		resource.ResourceVersion, resource.Data, resource.ResourceVersion, resource.Data)

	if err != nil {
		return fmt.Errorf("failed to upsert resource: %v", err)
	}

	return nil
}

// Delete removes a resource from the database
func (s *ResourceStore) Delete(kind, apiVersion, namespace, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`
		DELETE FROM resources
		WHERE kind = ? AND api_version = ? AND namespace = ? AND name = ?
	`, kind, apiVersion, namespace, name)

	if err != nil {
		return fmt.Errorf("failed to delete resource: %v", err)
	}

	return nil
}

// Search performs a fuzzy search for resources
func (s *ResourceStore) Search(query string) ([]Resource, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var resources []Resource

	var rows *sql.Rows
	var err error

	if query == "" {
		// Return everything when query is empty
		rows, err = s.db.Query(`
			SELECT id, name, namespace, kind, api_version, resource_version, data
			FROM resources
			ORDER BY namespace, kind, name
			LIMIT 100
		`)
	} else {
		// Use LIKE for simple pattern matching
		searchPattern := "%" + query + "%"
		rows, err = s.db.Query(`
			SELECT id, name, namespace, kind, api_version, resource_version, data
			FROM resources
			WHERE name LIKE ? OR namespace LIKE ? OR kind LIKE ?
			ORDER BY namespace, kind, name
			LIMIT 100
		`, searchPattern, searchPattern, searchPattern)
	}

	if err != nil {
		return nil, fmt.Errorf("search query failed: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var r Resource
		if err := rows.Scan(&r.ID, &r.Name, &r.Namespace, &r.Kind, &r.APIVersion, &r.ResourceVersion, &r.Data); err != nil {
			return nil, fmt.Errorf("failed to scan row: %v", err)
		}
		resources = append(resources, r)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %v", err)
	}

	return resources, nil
}

// ResourceCount returns the total number of resources in the database
func (s *ResourceStore) ResourceCount() (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM resources").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count resources: %v", err)
	}

	return count, nil
}

// CleanDatabase removes all resources from the database
func (s *ResourceStore) CleanDatabase() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec("DELETE FROM resources")
	if err != nil {
		return fmt.Errorf("failed to clean database: %v", err)
	}

	return nil
}

// Debug prints database statistics to the logger
func (s *ResourceStore) Debug() {
	count, err := s.ResourceCount()
	if err != nil {
		log.Printf("Failed to get resource count: %v", err)
		return
	}

	log.Printf("Database contains %d resources", count)
}

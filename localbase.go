// Package localbase implements a lightweight, file-based JSON document store.
//
// LocalBase provides thread-safe CRUD operations for storing structured data
// as JSON files on disk. It supports concurrent reads via read-write locks,
// atomic writes using temp-file-then-rename, collection management, batch
// operations, filtered queries, and collection-level statistics.
//
// Usage:
//
//	db, err := localbase.New("./data", nil)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer db.Close()
//
//	// Write a record
//	err = db.Write("users", "john", User{Name: "John", Age: 30})
//
//	// Read a record
//	var user User
//	err = db.Read("users", "john", &user)
//
//	// Query with filters
//	results, err := db.Where("users", localbase.FieldEquals("company", "Google"))
package localbase

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/jcelliott/lumber"
)

// Version is the current release version of LocalBase.
const Version = "1.0.0"

// Driver is the core database engine that manages file-based JSON storage.
//
// It provides thread-safe operations with per-collection read-write locking
// and atomic file writes (write-to-temp + rename) to prevent data corruption
// on crashes or power loss.
//
// A Driver instance is safe for concurrent use by multiple goroutines.
type Driver struct {
	mu      sync.Mutex                // protects the mutexes map itself
	mutexes map[string]*sync.RWMutex // per-collection read-write locks
	dir     string                    // root data directory
	log     Logger
}

// New creates and initializes a new LocalBase driver instance.
//
// The dir parameter specifies the root directory where collections and
// their JSON documents will be stored. If the directory does not exist,
// it will be created automatically with 0755 permissions.
//
// Options can be nil, in which case a default console logger (INFO level)
// is used.
//
// Returns an error if the directory path is empty or cannot be created.
func New(dir string, options *Options) (*Driver, error) {
	if dir == "" {
		return nil, ErrEmptyDir
	}
	dir = filepath.Clean(dir)

	opts := Options{}
	if options != nil {
		opts = *options
	}
	if opts.Logger == nil {
		opts.Logger = lumber.NewConsoleLogger(lumber.INFO)
	}

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("localbase: failed to create directory %q: %w", dir, err)
		}
	}

	driver := &Driver{
		dir:     dir,
		mutexes: make(map[string]*sync.RWMutex),
		log:     opts.Logger,
	}

	driver.log.Info("LocalBase v%s initialized at %s", Version, dir)
	return driver, nil
}

// Write stores a value in the specified collection with the given resource key.
//
// The data is serialized as indented JSON and written atomically: first to a
// temporary file, then renamed to the final path. This ensures that no partial
// writes are visible in case of a crash.
//
// If the collection directory does not exist, it is created automatically.
// If a resource with the same key already exists, it is overwritten.
func (d *Driver) Write(collection, resource string, v interface{}) error {
	if err := validateCollectionResource(collection, resource); err != nil {
		return err
	}

	mu := d.getOrCreateMutex(collection)
	mu.Lock()
	defer mu.Unlock()

	return d.writeUnsafe(collection, resource, v)
}

// Read retrieves and deserializes a JSON resource from the specified collection.
//
// The target parameter should be a pointer to the struct or type to unmarshal
// into. A read lock is acquired to allow concurrent reads while blocking
// writes to the same collection.
//
// Returns a ResourceNotFoundError if the resource does not exist.
func (d *Driver) Read(collection, resource string, target interface{}) error {
	if err := validateCollectionResource(collection, resource); err != nil {
		return err
	}

	mu := d.getOrCreateMutex(collection)
	mu.RLock()
	defer mu.RUnlock()

	recordPath := filepath.Join(d.dir, collection, resource+".json")

	if _, err := os.Stat(recordPath); os.IsNotExist(err) {
		return &ResourceNotFoundError{Collection: collection, Resource: resource}
	}

	b, err := os.ReadFile(recordPath)
	if err != nil {
		return fmt.Errorf("localbase: failed to read %s/%s: %w", collection, resource, err)
	}

	return json.Unmarshal(b, target)
}

// ReadAll retrieves all resources from the specified collection as raw JSON strings.
//
// A read lock is acquired for the duration of the operation to ensure
// consistency. Returns an empty slice if the collection exists but contains
// no resources.
//
// Returns a CollectionNotFoundError if the collection does not exist.
func (d *Driver) ReadAll(collection string) ([]string, error) {
	if collection == "" {
		return nil, ErrEmptyCollection
	}

	mu := d.getOrCreateMutex(collection)
	mu.RLock()
	defer mu.RUnlock()

	dir := filepath.Join(d.dir, collection)

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, &CollectionNotFoundError{Collection: collection}
	}

	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("localbase: failed to read collection %q: %w", collection, err)
	}

	records := make([]string, 0, len(files))
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".json") {
			continue
		}
		// Skip temporary files from in-progress writes
		if strings.HasSuffix(file.Name(), ".tmp") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, file.Name()))
		if err != nil {
			return nil, fmt.Errorf("localbase: failed to read file %s: %w", file.Name(), err)
		}
		records = append(records, string(b))
	}
	return records, nil
}

// Update modifies an existing resource using a caller-provided update function.
//
// The updateFn receives the current raw JSON bytes of the resource and should
// return the updated value to be written back. This operation is atomic — a
// write lock is held for the entire read-modify-write cycle to prevent race
// conditions.
//
// Example:
//
//	db.Update("users", "john", func(current []byte) (interface{}, error) {
//	    var user User
//	    json.Unmarshal(current, &user)
//	    user.Age = 31
//	    return user, nil
//	})
func (d *Driver) Update(collection, resource string, updateFn func(current []byte) (interface{}, error)) error {
	if err := validateCollectionResource(collection, resource); err != nil {
		return err
	}
	if updateFn == nil {
		return fmt.Errorf("localbase: update function cannot be nil")
	}

	mu := d.getOrCreateMutex(collection)
	mu.Lock()
	defer mu.Unlock()

	recordPath := filepath.Join(d.dir, collection, resource+".json")

	if _, err := os.Stat(recordPath); os.IsNotExist(err) {
		return &ResourceNotFoundError{Collection: collection, Resource: resource}
	}

	current, err := os.ReadFile(recordPath)
	if err != nil {
		return fmt.Errorf("localbase: failed to read %s/%s for update: %w", collection, resource, err)
	}

	updated, err := updateFn(current)
	if err != nil {
		return fmt.Errorf("localbase: update function failed: %w", err)
	}

	return d.writeUnsafe(collection, resource, updated)
}

// BatchWrite writes multiple resources to a collection in a single locked
// operation.
//
// All resources are written while holding the collection's write lock, ensuring
// that no reads or other writes can interleave. If any individual write fails,
// the error is returned immediately (previously written resources in this batch
// are NOT rolled back).
//
// Passing an empty map is a no-op and returns nil.
func (d *Driver) BatchWrite(collection string, resources map[string]interface{}) error {
	if collection == "" {
		return ErrEmptyCollection
	}
	if len(resources) == 0 {
		return nil
	}

	mu := d.getOrCreateMutex(collection)
	mu.Lock()
	defer mu.Unlock()

	for resource, value := range resources {
		if resource == "" {
			return ErrEmptyResource
		}
		if err := d.writeUnsafe(collection, resource, value); err != nil {
			return fmt.Errorf("localbase: batch write failed at resource %q: %w", resource, err)
		}
	}

	return nil
}

// Delete removes a specific resource from the specified collection.
//
// Returns a ResourceNotFoundError if the resource does not exist.
func (d *Driver) Delete(collection, resource string) error {
	if err := validateCollectionResource(collection, resource); err != nil {
		return err
	}

	mu := d.getOrCreateMutex(collection)
	mu.Lock()
	defer mu.Unlock()

	recordPath := filepath.Join(d.dir, collection, resource+".json")

	if _, err := os.Stat(recordPath); os.IsNotExist(err) {
		return &ResourceNotFoundError{Collection: collection, Resource: resource}
	}

	if err := os.Remove(recordPath); err != nil {
		return fmt.Errorf("localbase: failed to delete %s/%s: %w", collection, resource, err)
	}

	return nil
}

// DeleteAll removes all resources from the specified collection.
//
// The collection directory itself is preserved; only JSON files within
// the collection are deleted. Subdirectories are skipped.
//
// Returns a CollectionNotFoundError if the collection does not exist.
func (d *Driver) DeleteAll(collection string) error {
	if collection == "" {
		return ErrEmptyCollection
	}

	mu := d.getOrCreateMutex(collection)
	mu.Lock()
	defer mu.Unlock()

	dir := filepath.Join(d.dir, collection)

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return &CollectionNotFoundError{Collection: collection}
	}

	files, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("localbase: failed to read collection %q: %w", collection, err)
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}
		if err := os.Remove(filepath.Join(dir, file.Name())); err != nil {
			return fmt.Errorf("localbase: failed to delete file %s: %w", file.Name(), err)
		}
	}
	return nil
}

// DropCollection removes an entire collection including its directory
// and all contained resources.
//
// This is a destructive operation — all data in the collection will be
// permanently deleted. Returns a CollectionNotFoundError if the collection
// does not exist.
func (d *Driver) DropCollection(collection string) error {
	if collection == "" {
		return ErrEmptyCollection
	}

	mu := d.getOrCreateMutex(collection)
	mu.Lock()
	defer mu.Unlock()

	dir := filepath.Join(d.dir, collection)

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return &CollectionNotFoundError{Collection: collection}
	}

	return os.RemoveAll(dir)
}

// Collections returns a list of all collection names in the database.
//
// A collection is any subdirectory of the root data directory.
func (d *Driver) Collections() ([]string, error) {
	entries, err := os.ReadDir(d.dir)
	if err != nil {
		return nil, fmt.Errorf("localbase: failed to list collections: %w", err)
	}

	collections := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			collections = append(collections, entry.Name())
		}
	}
	return collections, nil
}

// Stats returns storage statistics for the specified collection, including
// document count and total size in bytes.
//
// Returns a CollectionNotFoundError if the collection does not exist.
func (d *Driver) Stats(collection string) (*CollectionStats, error) {
	if collection == "" {
		return nil, ErrEmptyCollection
	}

	mu := d.getOrCreateMutex(collection)
	mu.RLock()
	defer mu.RUnlock()

	dir := filepath.Join(d.dir, collection)

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, &CollectionNotFoundError{Collection: collection}
	}

	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("localbase: failed to stat collection %q: %w", collection, err)
	}

	stats := &CollectionStats{
		Collection: collection,
		Path:       dir,
	}

	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".json") {
			continue
		}
		info, err := file.Info()
		if err != nil {
			continue
		}
		stats.DocumentCount++
		stats.TotalSizeBytes += info.Size()
	}

	return stats, nil
}

// Close performs cleanup when the database is no longer needed.
//
// It clears all internal mutexes and logs the shutdown. After Close is called,
// the driver should not be used for further operations.
func (d *Driver) Close() error {
	d.log.Info("LocalBase closing...")
	d.mu.Lock()
	defer d.mu.Unlock()
	d.mutexes = make(map[string]*sync.RWMutex)
	d.log.Info("LocalBase closed.")
	return nil
}

// getOrCreateMutex returns a read-write mutex for the given collection.
// If no mutex exists for the collection, a new one is created and stored.
// This method is safe for concurrent access.
func (d *Driver) getOrCreateMutex(collection string) *sync.RWMutex {
	d.mu.Lock()
	defer d.mu.Unlock()
	if m, ok := d.mutexes[collection]; ok {
		return m
	}
	m := &sync.RWMutex{}
	d.mutexes[collection] = m
	return m
}

// writeUnsafe performs the actual atomic file write without acquiring locks.
// Callers MUST hold the collection's write lock before calling this method.
//
// The write is atomic: data is first written to a .tmp file, then renamed
// to the final .json path. If the rename fails, the temp file is cleaned up.
func (d *Driver) writeUnsafe(collection, resource string, v interface{}) error {
	collectionDir := filepath.Join(d.dir, collection)
	if err := os.MkdirAll(collectionDir, 0755); err != nil {
		return fmt.Errorf("localbase: failed to create collection dir: %w", err)
	}

	finalPath := filepath.Join(collectionDir, resource+".json")
	tmpPath := finalPath + ".tmp"

	b, err := json.MarshalIndent(v, "", "\t")
	if err != nil {
		return fmt.Errorf("localbase: failed to marshal data: %w", err)
	}
	b = append(b, '\n')

	if err := os.WriteFile(tmpPath, b, 0644); err != nil {
		return fmt.Errorf("localbase: failed to write temp file: %w", err)
	}

	if err := os.Rename(tmpPath, finalPath); err != nil {
		os.Remove(tmpPath) // cleanup on failure
		return fmt.Errorf("localbase: failed to commit write: %w", err)
	}

	return nil
}

// validateCollectionResource validates that both collection and resource names
// are non-empty.
func validateCollectionResource(collection, resource string) error {
	if collection == "" {
		return ErrEmptyCollection
	}
	if resource == "" {
		return ErrEmptyResource
	}
	return nil
}

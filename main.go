// Package main implements a simple file-based database system.
// It provides basic CRUD operations for storing JSON data in collections.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/jcelliott/lumber"
)

const version = "1.0.0"

// Logger interface defines the logging methods required by the database driver.
type Logger interface {
	Fatal(string, ...interface{})
	Error(string, ...interface{})
	Warn(string, ...interface{})
	Info(string, ...interface{})
	Debug(string, ...interface{})
	Trace(string, ...interface{})
}

// Driver represents the database driver instance.
// It manages file-based storage with thread-safe operations.
type Driver struct {
	mutex   sync.Mutex
	mutexes map[string]*sync.Mutex
	dir     string
	log     Logger
}

// Options contains configuration options for the database driver.
type Options struct {
	Logger Logger
}

// New creates a new database driver instance.
// It initializes the driver with the specified directory and options.
func New(dir string, options *Options) (*Driver, error) {
	// validate and prepare the directory
	if dir == "" {
		return nil, fmt.Errorf("directory path cannot be empty")
	}
	dir = filepath.Clean(dir)

	// set up options and logger
	opt := &Options{}
	if options != nil {
		opt = options
	}
	if opt.Logger == nil {
		l := lumber.NewConsoleLogger(lumber.INFO)
		opt.Logger = l
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory: %v", err)
		}
	}

	// create and return the Driver instance
	driver := &Driver{
		dir:     dir,
		mutexes: make(map[string]*sync.Mutex),
		log:     opt.Logger,
	}

	driver.log.Info("Database initialized with directory: %s", dir)
	return driver, nil
}

// Write stores a value in the specified collection with the given resource name.
// The data is stored as JSON in a file at collection/resource.json.
func (d *Driver) Write(collection, resource string, v interface{}) error {
	if collection == "" {
		return fmt.Errorf("collection name cannot be empty")
	}
	if resource == "" {
		return fmt.Errorf("resource name cannot be empty")
	}
	
	mutex := d.getOrCreateMutex(collection)
	mutex.Lock()
	defer mutex.Unlock()
	
	// Create collection directory if it doesn't exist
	collectionDir := filepath.Join(d.dir, collection)
	if err := os.MkdirAll(collectionDir, 0755); err != nil {
		return fmt.Errorf("failed to create collection directory: %v", err)
	}
	
	// Use proper file path: collection/resource.json
	finalPath := filepath.Join(collectionDir, resource+".json")
	tmpPath := finalPath + ".tmp"
	
	b, err := json.MarshalIndent(v, "", "\t")
	if err != nil {
		return fmt.Errorf("failed to marshal data: %v", err)
	}
	// Append a newline for better readability
	b = append(b, '\n')

	if err := os.WriteFile(tmpPath, b, 0644); err != nil {
		return fmt.Errorf("failed to write to temp file: %v", err)
	}	

	if err := os.Rename(tmpPath, finalPath); err != nil {
		return fmt.Errorf("failed to rename temp file: %v", err)
	}

	return nil
}

// Read retrieves a value from the specified collection and resource.
// The data is unmarshaled into the provided interface.
func (d *Driver) Read(collection, resource string, v interface{}) error {
	if collection == "" {
		return fmt.Errorf("collection name cannot be empty")
	}
	if resource == "" {
		return fmt.Errorf("resource name cannot be empty")
	}
	
	recordPath := filepath.Join(d.dir, collection, resource+".json")

	if _, err := os.Stat(recordPath); os.IsNotExist(err) {
		return fmt.Errorf("resource %s does not exist in collection %s", resource, collection)
	}

	b, err := os.ReadFile(recordPath)
	if err != nil {
		return fmt.Errorf("failed to read file: %v", err)
	}

	return json.Unmarshal(b, v)
}

// ReadAll retrieves all resources from the specified collection.
// Returns a slice of JSON strings representing all resources in the collection.
func (d *Driver) ReadAll(collection string) ([]string, error) {
	if collection == "" {
		return nil, fmt.Errorf("collection name cannot be empty")
	}
	dir := filepath.Join(d.dir, collection)
	
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, fmt.Errorf("collection %s does not exist", collection)
	}
	
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %v", err)
	}

	var records []string
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, file.Name()))
		if err != nil {
			return nil, fmt.Errorf("failed to read file %s: %v", file.Name(), err)
		}
		records = append(records, string(b))
	}
	return records, nil
}

// Delete removes a specific resource from the specified collection.
func (d *Driver) Delete(collection, resource string) error {
	if collection == "" {
		return fmt.Errorf("collection name cannot be empty")
	}
	if resource == "" {
		return fmt.Errorf("resource name cannot be empty")
	}
	
	mutex := d.getOrCreateMutex(collection)
	mutex.Lock()
	defer mutex.Unlock()
	
	recordPath := filepath.Join(d.dir, collection, resource+".json")

	if _, err := os.Stat(recordPath); os.IsNotExist(err) {
		return fmt.Errorf("resource %s does not exist in collection %s", resource, collection)
	}

	if err := os.Remove(recordPath); err != nil {
		return fmt.Errorf("failed to delete file: %v", err)
	}

	return nil
}

// DeleteAll removes all resources from the specified collection.
func (d *Driver) DeleteAll(collection string) error {
	if collection == "" {
		return fmt.Errorf("collection name cannot be empty")
	}
	dir := filepath.Join(d.dir, collection)

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("collection %s does not exist", collection)
	}

	files, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read directory: %v", err)
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}
		if err := os.Remove(filepath.Join(dir, file.Name())); err != nil {
			return fmt.Errorf("failed to delete file %s: %v", file.Name(), err)
		}
	}
	return nil
}

// getOrCreateMutex returns a mutex for the specified collection.
// If no mutex exists for the collection, a new one is created.
func (d *Driver) getOrCreateMutex(collection string) *sync.Mutex {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	if mutex, exists := d.mutexes[collection]; exists {
		return mutex
	}

	mutex := &sync.Mutex{}
	d.mutexes[collection] = mutex
	return mutex
}



// Close performs cleanup operations when the database is no longer needed.
func (d *Driver) Close() error {
	d.log.Info("Closing database...")
	// Perform any necessary cleanup here
	d.log.Info("Database closed.")
	return nil
}





// Address represents a user's address information.
type Address struct {
	Street  string `json:"street"`
	City    string `json:"city"`
	State   string `json:"state"`
	PinCode string `json:"pin_code"`
}

// User represents a user entity with personal and contact information.
type User struct {
	ID      int     `json:"id"`
	Name    string  `json:"name"`
	Age     int     `json:"age"`
	Contact string  `json:"contact"`
	Company string  `json:"company"`
	Address Address `json:"address"`
}


func main() {
	fmt.Println("DB initialization started...")
	dir := "./data"

	db, err := New(dir, nil)
	if err != nil {
		fmt.Println("Error in DB initialization:", err)
		return
	}
	defer db.Close()
	fmt.Println("DB initialized successfully.")

	// Create sample users
	employees := []User{
		{ID: 1, Name: "Alice", Age: 30, Contact: "alice@example.com", Company: "TechCorp", Address: Address{Street: "123 Tech Lane", City: "Tech City", State: "CA", PinCode: "12345"}},
		{ID: 2, Name: "Bob", Age: 25, Contact: "bob@example.com", Company: "Innovate LLC", Address: Address{Street: "456 Innovation Blvd", City: "Innovation City", State: "NY", PinCode: "67890"}},
		{ID: 3, Name: "Charlie", Age: 35, Contact: "charlie@example.com", Company: "Creative Inc", Address: Address{Street: "789 Creativity St", City: "Creative City", State: "TX", PinCode: "54321"}},
	}

	// Write users to database
	for _, emp := range employees {
		err = db.Write("users", emp.Name, emp)
		if err != nil {
			fmt.Printf("Error creating user %s: %v\n", emp.Name, err)
			continue
		}
		fmt.Printf("User %s created successfully.\n", emp.Name)
	}

	// Read all users
	users, err := db.ReadAll("users")
	if err != nil {
		fmt.Println("Error reading users:", err)
		return
	}
	fmt.Println("\nAll Users:")
	for _, userJSON := range users {
		var u User
		err := json.Unmarshal([]byte(userJSON), &u)
		if err != nil {
			fmt.Println("Error unmarshalling user data:", err)
			continue
		}
		fmt.Printf("%+v\n", u)
	}

	// Delete a specific user
	err = db.Delete("users", "Bob")
	if err != nil {
		fmt.Println("Error deleting user:", err)
		return
	}
	fmt.Println("\nUser Bob deleted successfully.")

	// Read remaining users
	fmt.Println("\nRemaining users after deletion:")
	users, err = db.ReadAll("users")
	if err != nil {
		fmt.Println("Error reading users:", err)
		return
	}
	for _, userJSON := range users {
		var u User
		err := json.Unmarshal([]byte(userJSON), &u)
		if err != nil {
			fmt.Println("Error unmarshalling user data:", err)
			continue
		}
		fmt.Printf("%+v\n", u)
	}

	// Comment out DeleteAll to keep files for inspection
	// err = db.DeleteAll("users")
	// if err != nil {
	// 	fmt.Println("Error deleting all users:", err)
	// 	return
	// }
	// fmt.Println("\nAll users deleted successfully.")
	
	fmt.Println("\nFiles created successfully. Check the data/users/ directory for JSON files.")
}
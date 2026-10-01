package localbase

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// testUser is a sample struct used across all tests.
type testUser struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Age     int    `json:"age"`
	Email   string `json:"email"`
	Company string `json:"company"`
}

// setupTestDB creates a temporary database for testing.
// The directory is automatically cleaned up after the test.
func setupTestDB(t *testing.T) (*Driver, string) {
	t.Helper()
	dir := t.TempDir()
	db, err := New(dir, &Options{Logger: &noopLogger{}})
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}
	return db, dir
}

// --- New ---

func TestNew(t *testing.T) {
	t.Run("creates database with valid directory", func(t *testing.T) {
		db, _ := setupTestDB(t)
		defer db.Close()
		if db == nil {
			t.Fatal("expected non-nil driver")
		}
	})

	t.Run("creates nested directories if they don't exist", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "nested", "deep", "dir")
		db, err := New(dir, &Options{Logger: &noopLogger{}})
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		defer db.Close()

		info, err := os.Stat(dir)
		if err != nil {
			t.Fatalf("directory was not created: %v", err)
		}
		if !info.IsDir() {
			t.Fatal("expected a directory")
		}
	})

	t.Run("rejects empty directory path", func(t *testing.T) {
		_, err := New("", nil)
		if err != ErrEmptyDir {
			t.Fatalf("expected ErrEmptyDir, got: %v", err)
		}
	})

	t.Run("accepts nil options and uses defaults", func(t *testing.T) {
		dir := t.TempDir()
		db, err := New(dir, nil)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		defer db.Close()
	})

	t.Run("accepts custom logger", func(t *testing.T) {
		dir := t.TempDir()
		opts := &Options{Logger: &noopLogger{}}
		db, err := New(dir, opts)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		defer db.Close()
	})
}

// --- Write ---

func TestWrite(t *testing.T) {
	db, dir := setupTestDB(t)
	defer db.Close()

	user := testUser{ID: 1, Name: "Alice", Age: 30, Email: "alice@test.com"}

	t.Run("writes record and creates JSON file", func(t *testing.T) {
		err := db.Write("users", "alice", user)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		path := filepath.Join(dir, "users", "alice.json")
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Fatal("expected JSON file to be created")
		}
	})

	t.Run("creates collection directory automatically", func(t *testing.T) {
		err := db.Write("products", "item1", map[string]string{"key": "value"})
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		path := filepath.Join(dir, "products")
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("collection directory was not created: %v", err)
		}
		if !info.IsDir() {
			t.Fatal("expected a directory")
		}
	})

	t.Run("overwrites existing record", func(t *testing.T) {
		original := testUser{ID: 1, Name: "Alice", Age: 30}
		updated := testUser{ID: 1, Name: "Alice", Age: 31}

		db.Write("users", "alice_v2", original)
		err := db.Write("users", "alice_v2", updated)
		if err != nil {
			t.Fatalf("expected no error on overwrite, got: %v", err)
		}

		var result testUser
		db.Read("users", "alice_v2", &result)
		if result.Age != 31 {
			t.Fatalf("expected age 31 after overwrite, got %d", result.Age)
		}
	})

	t.Run("writes valid JSON", func(t *testing.T) {
		db.Write("users", "json_check", user)
		path := filepath.Join(dir, "users", "json_check.json")
		b, _ := os.ReadFile(path)

		var parsed testUser
		if err := json.Unmarshal(b, &parsed); err != nil {
			t.Fatalf("file does not contain valid JSON: %v", err)
		}
		if parsed.Name != "Alice" {
			t.Fatalf("expected name Alice, got %s", parsed.Name)
		}
	})

	t.Run("rejects empty collection name", func(t *testing.T) {
		err := db.Write("", "resource", user)
		if err != ErrEmptyCollection {
			t.Fatalf("expected ErrEmptyCollection, got: %v", err)
		}
	})

	t.Run("rejects empty resource name", func(t *testing.T) {
		err := db.Write("users", "", user)
		if err != ErrEmptyResource {
			t.Fatalf("expected ErrEmptyResource, got: %v", err)
		}
	})
}

// --- Read ---

func TestRead(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()

	user := testUser{ID: 1, Name: "Bob", Age: 25, Email: "bob@test.com", Company: "TestCo"}
	db.Write("users", "bob", user)

	t.Run("reads record with all fields", func(t *testing.T) {
		var result testUser
		err := db.Read("users", "bob", &result)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if result.Name != "Bob" {
			t.Fatalf("expected name Bob, got %s", result.Name)
		}
		if result.Age != 25 {
			t.Fatalf("expected age 25, got %d", result.Age)
		}
		if result.Email != "bob@test.com" {
			t.Fatalf("expected email bob@test.com, got %s", result.Email)
		}
		if result.Company != "TestCo" {
			t.Fatalf("expected company TestCo, got %s", result.Company)
		}
	})

	t.Run("returns ResourceNotFoundError for missing resource", func(t *testing.T) {
		var result testUser
		err := db.Read("users", "nonexistent", &result)
		if !IsNotFound(err) {
			t.Fatalf("expected not found error, got: %v", err)
		}

		var rnf *ResourceNotFoundError
		if !errors.As(err, &rnf) {
			t.Fatal("expected ResourceNotFoundError type")
		}
		if rnf.Resource != "nonexistent" {
			t.Fatalf("expected resource 'nonexistent', got %q", rnf.Resource)
		}
	})

	t.Run("returns ResourceNotFoundError for missing collection", func(t *testing.T) {
		var result testUser
		err := db.Read("ghost_collection", "resource", &result)
		if !IsNotFound(err) {
			t.Fatalf("expected not found error, got: %v", err)
		}
	})

	t.Run("rejects empty collection name", func(t *testing.T) {
		var result testUser
		err := db.Read("", "bob", &result)
		if err != ErrEmptyCollection {
			t.Fatalf("expected ErrEmptyCollection, got: %v", err)
		}
	})

	t.Run("rejects empty resource name", func(t *testing.T) {
		var result testUser
		err := db.Read("users", "", &result)
		if err != ErrEmptyResource {
			t.Fatalf("expected ErrEmptyResource, got: %v", err)
		}
	})
}

// --- ReadAll ---

func TestReadAll(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()

	users := []testUser{
		{ID: 1, Name: "Alice", Age: 30},
		{ID: 2, Name: "Bob", Age: 25},
		{ID: 3, Name: "Charlie", Age: 35},
	}
	for _, u := range users {
		db.Write("users", u.Name, u)
	}

	t.Run("reads all records in collection", func(t *testing.T) {
		records, err := db.ReadAll("users")
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if len(records) != 3 {
			t.Fatalf("expected 3 records, got %d", len(records))
		}
	})

	t.Run("each record is valid JSON", func(t *testing.T) {
		records, _ := db.ReadAll("users")
		for i, raw := range records {
			var u testUser
			if err := json.Unmarshal([]byte(raw), &u); err != nil {
				t.Fatalf("record %d is not valid JSON: %v", i, err)
			}
		}
	})

	t.Run("returns error for non-existent collection", func(t *testing.T) {
		_, err := db.ReadAll("nonexistent")
		if !IsNotFound(err) {
			t.Fatalf("expected not found error, got: %v", err)
		}
	})

	t.Run("returns empty slice for empty collection", func(t *testing.T) {
		// Create collection dir without any files
		os.MkdirAll(filepath.Join(db.dir, "empty_col"), 0755)
		records, err := db.ReadAll("empty_col")
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if len(records) != 0 {
			t.Fatalf("expected 0 records, got %d", len(records))
		}
	})

	t.Run("rejects empty collection name", func(t *testing.T) {
		_, err := db.ReadAll("")
		if err != ErrEmptyCollection {
			t.Fatalf("expected ErrEmptyCollection, got: %v", err)
		}
	})
}

// --- Update ---

func TestUpdate(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()

	original := testUser{ID: 1, Name: "Alice", Age: 30, Email: "alice@test.com", Company: "OldCo"}
	db.Write("users", "alice", original)

	t.Run("updates specific fields", func(t *testing.T) {
		err := db.Update("users", "alice", func(current []byte) (interface{}, error) {
			var u testUser
			if err := json.Unmarshal(current, &u); err != nil {
				return nil, err
			}
			u.Age = 31
			u.Email = "alice.new@test.com"
			return u, nil
		})
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		var result testUser
		db.Read("users", "alice", &result)
		if result.Age != 31 {
			t.Fatalf("expected age 31, got %d", result.Age)
		}
		if result.Email != "alice.new@test.com" {
			t.Fatalf("expected updated email, got %s", result.Email)
		}
	})

	t.Run("preserves unchanged fields", func(t *testing.T) {
		var result testUser
		db.Read("users", "alice", &result)
		if result.Name != "Alice" {
			t.Fatalf("expected name Alice to be preserved, got %s", result.Name)
		}
		if result.ID != 1 {
			t.Fatalf("expected ID 1 to be preserved, got %d", result.ID)
		}
	})

	t.Run("returns error for non-existent resource", func(t *testing.T) {
		err := db.Update("users", "ghost", func(current []byte) (interface{}, error) {
			return nil, nil
		})
		if !IsNotFound(err) {
			t.Fatalf("expected not found error, got: %v", err)
		}
	})

	t.Run("propagates update function errors", func(t *testing.T) {
		err := db.Update("users", "alice", func(current []byte) (interface{}, error) {
			return nil, fmt.Errorf("intentional error")
		})
		if err == nil {
			t.Fatal("expected error from update function")
		}
	})

	t.Run("rejects nil update function", func(t *testing.T) {
		err := db.Update("users", "alice", nil)
		if err == nil {
			t.Fatal("expected error for nil update function")
		}
	})
}

// --- Delete ---

func TestDelete(t *testing.T) {
	db, dir := setupTestDB(t)
	defer db.Close()

	db.Write("users", "alice", testUser{Name: "Alice"})

	t.Run("deletes record and removes file", func(t *testing.T) {
		err := db.Delete("users", "alice")
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		path := filepath.Join(dir, "users", "alice.json")
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatal("expected file to be removed")
		}
	})

	t.Run("reading deleted record returns not found", func(t *testing.T) {
		var result testUser
		err := db.Read("users", "alice", &result)
		if !IsNotFound(err) {
			t.Fatal("expected not found after delete")
		}
	})

	t.Run("returns error for non-existent resource", func(t *testing.T) {
		err := db.Delete("users", "nonexistent")
		if !IsNotFound(err) {
			t.Fatalf("expected not found error, got: %v", err)
		}
	})

	t.Run("rejects empty arguments", func(t *testing.T) {
		if err := db.Delete("", "x"); err != ErrEmptyCollection {
			t.Fatalf("expected ErrEmptyCollection, got: %v", err)
		}
		if err := db.Delete("x", ""); err != ErrEmptyResource {
			t.Fatalf("expected ErrEmptyResource, got: %v", err)
		}
	})
}

// --- DeleteAll ---

func TestDeleteAll(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()

	db.Write("users", "alice", testUser{Name: "Alice"})
	db.Write("users", "bob", testUser{Name: "Bob"})
	db.Write("users", "charlie", testUser{Name: "Charlie"})

	t.Run("deletes all records but preserves directory", func(t *testing.T) {
		err := db.DeleteAll("users")
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		records, err := db.ReadAll("users")
		if err != nil {
			t.Fatalf("expected no error reading empty collection, got: %v", err)
		}
		if len(records) != 0 {
			t.Fatalf("expected 0 records after DeleteAll, got %d", len(records))
		}

		// Directory should still exist
		info, err := os.Stat(filepath.Join(db.dir, "users"))
		if err != nil {
			t.Fatal("expected collection directory to still exist")
		}
		if !info.IsDir() {
			t.Fatal("expected a directory")
		}
	})

	t.Run("returns error for non-existent collection", func(t *testing.T) {
		err := db.DeleteAll("nonexistent")
		if !IsNotFound(err) {
			t.Fatalf("expected not found error, got: %v", err)
		}
	})
}

// --- DropCollection ---

func TestDropCollection(t *testing.T) {
	db, dir := setupTestDB(t)
	defer db.Close()

	db.Write("temp", "item1", map[string]string{"key": "value"})
	db.Write("temp", "item2", map[string]string{"key": "value2"})

	t.Run("removes entire collection directory", func(t *testing.T) {
		err := db.DropCollection("temp")
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		path := filepath.Join(dir, "temp")
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatal("expected collection directory to be removed")
		}
	})

	t.Run("returns error for non-existent collection", func(t *testing.T) {
		err := db.DropCollection("nonexistent")
		if !IsNotFound(err) {
			t.Fatalf("expected not found error, got: %v", err)
		}
	})

	t.Run("rejects empty collection name", func(t *testing.T) {
		err := db.DropCollection("")
		if err != ErrEmptyCollection {
			t.Fatalf("expected ErrEmptyCollection, got: %v", err)
		}
	})
}

// --- BatchWrite ---

func TestBatchWrite(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()

	t.Run("writes multiple records atomically", func(t *testing.T) {
		resources := map[string]interface{}{
			"alice":   testUser{ID: 1, Name: "Alice", Age: 30},
			"bob":     testUser{ID: 2, Name: "Bob", Age: 25},
			"charlie": testUser{ID: 3, Name: "Charlie", Age: 35},
		}

		err := db.BatchWrite("batch_test", resources)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		records, _ := db.ReadAll("batch_test")
		if len(records) != 3 {
			t.Fatalf("expected 3 records, got %d", len(records))
		}
	})

	t.Run("each batch record is readable individually", func(t *testing.T) {
		var alice testUser
		err := db.Read("batch_test", "alice", &alice)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if alice.Name != "Alice" {
			t.Fatalf("expected name Alice, got %s", alice.Name)
		}
	})

	t.Run("handles empty batch gracefully", func(t *testing.T) {
		err := db.BatchWrite("batch_test", map[string]interface{}{})
		if err != nil {
			t.Fatalf("expected no error for empty batch, got: %v", err)
		}
	})

	t.Run("rejects empty collection name", func(t *testing.T) {
		err := db.BatchWrite("", map[string]interface{}{"a": "b"})
		if err != ErrEmptyCollection {
			t.Fatalf("expected ErrEmptyCollection, got: %v", err)
		}
	})
}

// --- Collections ---

func TestCollections(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()

	db.Write("users", "alice", testUser{Name: "Alice"})
	db.Write("products", "laptop", map[string]string{"name": "Laptop"})
	db.Write("orders", "order1", map[string]int{"total": 100})

	t.Run("lists all collections", func(t *testing.T) {
		collections, err := db.Collections()
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if len(collections) != 3 {
			t.Fatalf("expected 3 collections, got %d: %v", len(collections), collections)
		}
	})

	t.Run("returns empty slice for empty database", func(t *testing.T) {
		emptyDB, _ := setupTestDB(t)
		defer emptyDB.Close()

		collections, err := emptyDB.Collections()
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if len(collections) != 0 {
			t.Fatalf("expected 0 collections, got %d", len(collections))
		}
	})
}

// --- Stats ---

func TestStats(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()

	db.Write("users", "alice", testUser{Name: "Alice", Age: 30})
	db.Write("users", "bob", testUser{Name: "Bob", Age: 25})

	t.Run("returns correct document count", func(t *testing.T) {
		stats, err := db.Stats("users")
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if stats.DocumentCount != 2 {
			t.Fatalf("expected 2 documents, got %d", stats.DocumentCount)
		}
	})

	t.Run("returns positive total size", func(t *testing.T) {
		stats, _ := db.Stats("users")
		if stats.TotalSizeBytes <= 0 {
			t.Fatal("expected positive total size")
		}
	})

	t.Run("returns correct collection name", func(t *testing.T) {
		stats, _ := db.Stats("users")
		if stats.Collection != "users" {
			t.Fatalf("expected collection 'users', got %q", stats.Collection)
		}
	})

	t.Run("String() format is readable", func(t *testing.T) {
		stats, _ := db.Stats("users")
		s := stats.String()
		if s == "" {
			t.Fatal("expected non-empty string representation")
		}
	})

	t.Run("returns error for non-existent collection", func(t *testing.T) {
		_, err := db.Stats("nonexistent")
		if !IsNotFound(err) {
			t.Fatalf("expected not found error, got: %v", err)
		}
	})

	t.Run("rejects empty collection name", func(t *testing.T) {
		_, err := db.Stats("")
		if err != ErrEmptyCollection {
			t.Fatalf("expected ErrEmptyCollection, got: %v", err)
		}
	})
}

// --- Where / Query ---

func TestWhere(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()

	users := []testUser{
		{ID: 1, Name: "Alice", Age: 30, Company: "TechCorp"},
		{ID: 2, Name: "Bob", Age: 25, Company: "TechCorp"},
		{ID: 3, Name: "Charlie", Age: 35, Company: "StartupInc"},
		{ID: 4, Name: "Diana", Age: 22, Company: "StartupInc"},
	}
	for _, u := range users {
		db.Write("users", u.Name, u)
	}

	t.Run("filters with custom function", func(t *testing.T) {
		results, err := db.Where("users", func(raw []byte) bool {
			var u testUser
			json.Unmarshal(raw, &u)
			return u.Age > 28
		})
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if len(results) != 2 {
			t.Fatalf("expected 2 results (Alice, Charlie), got %d", len(results))
		}
	})

	t.Run("filters with FieldEquals helper", func(t *testing.T) {
		results, err := db.Where("users", FieldEquals("company", "TechCorp"))
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if len(results) != 2 {
			t.Fatalf("expected 2 TechCorp results, got %d", len(results))
		}
	})

	t.Run("filters with FieldContains helper", func(t *testing.T) {
		results, err := db.Where("users", FieldContains("name", "li"))
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		// "Alice" and "Charlie" both contain "li"
		if len(results) != 2 {
			t.Fatalf("expected 2 results, got %d", len(results))
		}
	})

	t.Run("FieldContains is case-insensitive", func(t *testing.T) {
		results, err := db.Where("users", FieldContains("name", "ALICE"))
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
	})

	t.Run("nil filter returns all records", func(t *testing.T) {
		results, err := db.Where("users", nil)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if len(results) != 4 {
			t.Fatalf("expected 4 results, got %d", len(results))
		}
	})

	t.Run("returns empty slice when no match", func(t *testing.T) {
		results, err := db.Where("users", func(raw []byte) bool {
			var u testUser
			json.Unmarshal(raw, &u)
			return u.Age > 100
		})
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if len(results) != 0 {
			t.Fatalf("expected 0 results, got %d", len(results))
		}
	})

	t.Run("returns error for non-existent collection", func(t *testing.T) {
		_, err := db.Where("nonexistent", nil)
		if !IsNotFound(err) {
			t.Fatalf("expected not found error, got: %v", err)
		}
	})
}

// --- Count ---

func TestCount(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()

	db.Write("users", "alice", testUser{Name: "Alice", Age: 30})
	db.Write("users", "bob", testUser{Name: "Bob", Age: 25})
	db.Write("users", "charlie", testUser{Name: "Charlie", Age: 35})

	t.Run("counts all records with nil filter", func(t *testing.T) {
		count, err := db.Count("users", nil)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if count != 3 {
			t.Fatalf("expected 3, got %d", count)
		}
	})

	t.Run("counts filtered records", func(t *testing.T) {
		count, err := db.Count("users", func(raw []byte) bool {
			var u testUser
			json.Unmarshal(raw, &u)
			return u.Age >= 30
		})
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if count != 2 {
			t.Fatalf("expected 2, got %d", count)
		}
	})
}

// --- Concurrency ---

func TestConcurrentReadWrite(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()

	// Seed a record that readers will target
	db.Write("concurrent", "seed", testUser{Name: "Seed", Age: 0})

	var wg sync.WaitGroup
	errCh := make(chan error, 300)

	// 50 concurrent writers
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			user := testUser{ID: id, Name: fmt.Sprintf("user_%d", id), Age: id}
			if err := db.Write("concurrent", fmt.Sprintf("user_%d", id), user); err != nil {
				errCh <- fmt.Errorf("write error (id=%d): %w", id, err)
			}
		}(i)
	}

	// 50 concurrent readers (point reads)
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var u testUser
			if err := db.Read("concurrent", "seed", &u); err != nil {
				errCh <- fmt.Errorf("read error: %w", err)
			}
		}()
	}

	// 50 concurrent ReadAll
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := db.ReadAll("concurrent"); err != nil {
				errCh <- fmt.Errorf("readall error: %w", err)
			}
		}()
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("concurrent operation failed: %v", err)
	}

	// Verify all writes landed
	records, err := db.ReadAll("concurrent")
	if err != nil {
		t.Fatalf("failed to read all after concurrent ops: %v", err)
	}
	// 50 writers + 1 seed = 51
	if len(records) != 51 {
		t.Fatalf("expected 51 records after concurrent writes, got %d", len(records))
	}
}

func TestConcurrentUpdates(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()

	db.Write("counters", "hits", map[string]int{"count": 0})

	var wg sync.WaitGroup
	iterations := 100

	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			db.Update("counters", "hits", func(current []byte) (interface{}, error) {
				var m map[string]interface{}
				json.Unmarshal(current, &m)
				count := int(m["count"].(float64))
				m["count"] = count + 1
				return m, nil
			})
		}()
	}

	wg.Wait()

	var result map[string]interface{}
	db.Read("counters", "hits", &result)
	finalCount := int(result["count"].(float64))
	if finalCount != iterations {
		t.Fatalf("expected count %d after concurrent updates, got %d", iterations, finalCount)
	}
}

// --- Error types ---

func TestIsNotFound(t *testing.T) {
	t.Run("matches CollectionNotFoundError", func(t *testing.T) {
		err := &CollectionNotFoundError{Collection: "test"}
		if !IsNotFound(err) {
			t.Fatal("expected IsNotFound to return true")
		}
	})

	t.Run("matches ResourceNotFoundError", func(t *testing.T) {
		err := &ResourceNotFoundError{Collection: "test", Resource: "item"}
		if !IsNotFound(err) {
			t.Fatal("expected IsNotFound to return true")
		}
	})

	t.Run("does not match other errors", func(t *testing.T) {
		err := fmt.Errorf("some other error")
		if IsNotFound(err) {
			t.Fatal("expected IsNotFound to return false for unrelated error")
		}
	})

	t.Run("does not match nil", func(t *testing.T) {
		if IsNotFound(nil) {
			t.Fatal("expected IsNotFound to return false for nil")
		}
	})
}

func TestErrorMessages(t *testing.T) {
	t.Run("CollectionNotFoundError message", func(t *testing.T) {
		err := &CollectionNotFoundError{Collection: "users"}
		expected := `localbase: collection "users" not found`
		if err.Error() != expected {
			t.Fatalf("expected %q, got %q", expected, err.Error())
		}
	})

	t.Run("ResourceNotFoundError message", func(t *testing.T) {
		err := &ResourceNotFoundError{Collection: "users", Resource: "alice"}
		expected := `localbase: resource "alice" not found in collection "users"`
		if err.Error() != expected {
			t.Fatalf("expected %q, got %q", expected, err.Error())
		}
	})
}

// --- Close ---

func TestClose(t *testing.T) {
	db, _ := setupTestDB(t)

	err := db.Close()
	if err != nil {
		t.Fatalf("expected no error on close, got: %v", err)
	}
}

// --- Helpers ---

// noopLogger implements Logger but does nothing (used for quiet tests).
type noopLogger struct{}

func (l *noopLogger) Fatal(string, ...interface{}) {}
func (l *noopLogger) Error(string, ...interface{}) {}
func (l *noopLogger) Warn(string, ...interface{})  {}
func (l *noopLogger) Info(string, ...interface{})   {}
func (l *noopLogger) Debug(string, ...interface{})  {}
func (l *noopLogger) Trace(string, ...interface{})  {}

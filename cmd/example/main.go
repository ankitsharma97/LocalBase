// Command example demonstrates the full feature set of LocalBase.
//
// Run with: go run ./cmd/example
package main

import (
	"encoding/json"
	"fmt"
	"log"

	localbase "github.com/ankitsharma97/localbase"
)

// Address represents a user's address.
type Address struct {
	Street  string `json:"street"`
	City    string `json:"city"`
	State   string `json:"state"`
	PinCode string `json:"pin_code"`
}

// User represents a user entity.
type User struct {
	ID      int     `json:"id"`
	Name    string  `json:"name"`
	Age     int     `json:"age"`
	Contact string  `json:"contact"`
	Company string  `json:"company"`
	Address Address `json:"address"`
}

func main() {
	fmt.Println("🚀 LocalBase — File-Based JSON Document Store")
	fmt.Println("================================================")

	// ─── INITIALIZE ─────────────────────────────────────────
	db, err := localbase.New("./example_data", nil)
	if err != nil {
		log.Fatal("Failed to initialize database:", err)
	}
	defer db.Close()

	// ─── CREATE ─────────────────────────────────────────────
	fmt.Println("\n📝 Writing records...")
	employees := []User{
		{ID: 1, Name: "Alice", Age: 30, Contact: "alice@example.com", Company: "TechCorp",
			Address: Address{Street: "123 Tech Lane", City: "San Francisco", State: "CA", PinCode: "94102"}},
		{ID: 2, Name: "Bob", Age: 25, Contact: "bob@example.com", Company: "Innovate LLC",
			Address: Address{Street: "456 Innovation Blvd", City: "New York", State: "NY", PinCode: "10001"}},
		{ID: 3, Name: "Charlie", Age: 35, Contact: "charlie@example.com", Company: "Creative Inc",
			Address: Address{Street: "789 Creativity St", City: "Austin", State: "TX", PinCode: "73301"}},
		{ID: 4, Name: "Diana", Age: 28, Contact: "diana@example.com", Company: "TechCorp",
			Address: Address{Street: "321 Code Ave", City: "Seattle", State: "WA", PinCode: "98101"}},
	}

	for _, emp := range employees {
		if err := db.Write("users", emp.Name, emp); err != nil {
			fmt.Printf("   ❌ Error writing %s: %v\n", emp.Name, err)
			continue
		}
		fmt.Printf("   ✅ Created: %s\n", emp.Name)
	}

	// ─── READ ALL ───────────────────────────────────────────
	fmt.Println("\n📖 Reading all users...")
	records, err := db.ReadAll("users")
	if err != nil {
		log.Fatal("Failed to read users:", err)
	}
	for _, raw := range records {
		var u User
		json.Unmarshal([]byte(raw), &u)
		fmt.Printf("   👤 %s (age: %d, company: %s, city: %s)\n", u.Name, u.Age, u.Company, u.Address.City)
	}

	// ─── QUERY / FILTER ─────────────────────────────────────
	fmt.Println("\n🔍 Query: users at TechCorp...")
	techUsers, err := db.Where("users", localbase.FieldEquals("company", "TechCorp"))
	if err != nil {
		log.Fatal("Query failed:", err)
	}
	for _, raw := range techUsers {
		var u User
		json.Unmarshal([]byte(raw), &u)
		fmt.Printf("   🏢 %s — %s\n", u.Name, u.Company)
	}

	fmt.Println("\n🔍 Query: users older than 28 (custom filter)...")
	seniorUsers, err := db.Where("users", func(raw []byte) bool {
		var u User
		json.Unmarshal(raw, &u)
		return u.Age > 28
	})
	if err != nil {
		log.Fatal("Query failed:", err)
	}
	for _, raw := range seniorUsers {
		var u User
		json.Unmarshal([]byte(raw), &u)
		fmt.Printf("   🎂 %s (age: %d)\n", u.Name, u.Age)
	}

	fmt.Println("\n🔍 Query: name contains 'li' (case-insensitive)...")
	liUsers, err := db.Where("users", localbase.FieldContains("name", "li"))
	if err != nil {
		log.Fatal("Query failed:", err)
	}
	for _, raw := range liUsers {
		var u User
		json.Unmarshal([]byte(raw), &u)
		fmt.Printf("   🔤 %s\n", u.Name)
	}

	// ─── UPDATE ─────────────────────────────────────────────
	fmt.Println("\n✏️  Updating Alice's age to 31...")
	err = db.Update("users", "Alice", func(current []byte) (interface{}, error) {
		var u User
		if err := json.Unmarshal(current, &u); err != nil {
			return nil, err
		}
		u.Age = 31
		return u, nil
	})
	if err != nil {
		log.Fatal("Update failed:", err)
	}

	var alice User
	db.Read("users", "Alice", &alice)
	fmt.Printf("   ✅ Alice's new age: %d\n", alice.Age)

	// ─── COUNT ──────────────────────────────────────────────
	total, _ := db.Count("users", nil)
	fmt.Printf("\n📊 Total users: %d\n", total)

	techCount, _ := db.Count("users", localbase.FieldEquals("company", "TechCorp"))
	fmt.Printf("📊 TechCorp employees: %d\n", techCount)

	// ─── STATS ──────────────────────────────────────────────
	stats, _ := db.Stats("users")
	fmt.Printf("📈 %s\n", stats)

	// ─── BATCH WRITE ────────────────────────────────────────
	fmt.Println("\n📦 Batch writing log entries...")
	logs := map[string]interface{}{
		"log_001": map[string]string{"level": "INFO", "message": "System started", "ts": "2024-01-01T00:00:00Z"},
		"log_002": map[string]string{"level": "WARN", "message": "Disk usage at 80%", "ts": "2024-01-01T01:00:00Z"},
		"log_003": map[string]string{"level": "ERROR", "message": "Connection timeout", "ts": "2024-01-01T02:00:00Z"},
	}
	if err := db.BatchWrite("logs", logs); err != nil {
		log.Fatal("Batch write failed:", err)
	}
	logCount, _ := db.Count("logs", nil)
	fmt.Printf("   ✅ Written %d log entries\n", logCount)

	// ─── COLLECTIONS ────────────────────────────────────────
	collections, _ := db.Collections()
	fmt.Printf("\n📁 All collections: %v\n", collections)

	// ─── DELETE ─────────────────────────────────────────────
	fmt.Println("\n🗑️  Deleting Bob...")
	if err := db.Delete("users", "Bob"); err != nil {
		log.Fatal("Delete failed:", err)
	}
	remaining, _ := db.ReadAll("users")
	fmt.Printf("   ✅ Remaining users: %d\n", len(remaining))

	// ─── ERROR HANDLING ─────────────────────────────────────
	fmt.Println("\n⚠️  Demonstrating error handling...")
	var ghost User
	err = db.Read("users", "NonExistent", &ghost)
	if localbase.IsNotFound(err) {
		fmt.Printf("   ✅ Correctly caught: %v\n", err)
	}

	// ─── DROP COLLECTION ────────────────────────────────────
	fmt.Println("\n💥 Dropping 'logs' collection...")
	if err := db.DropCollection("logs"); err != nil {
		log.Fatal("Drop failed:", err)
	}
	fmt.Println("   ✅ 'logs' collection dropped")

	fmt.Println("\n✨ Demo complete! Check the ./example_data directory for the persisted files.")
}

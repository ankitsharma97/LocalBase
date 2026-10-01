<p align="center">
  <h1 align="center">📦 LocalBase</h1>
  <p align="center">A lightweight, thread-safe, file-based JSON document store written in Go.</p>
</p>

<p align="center">
  <a href="https://github.com/ankitsharma97/localbase/actions"><img src="https://github.com/ankitsharma97/localbase/workflows/CI/badge.svg" alt="CI"></a>
  <a href="https://goreportcard.com/report/github.com/ankitsharma97/localbase"><img src="https://goreportcard.com/badge/github.com/ankitsharma97/localbase" alt="Go Report Card"></a>
  <a href="https://pkg.go.dev/github.com/ankitsharma97/localbase"><img src="https://pkg.go.dev/badge/github.com/ankitsharma97/localbase.svg" alt="Go Reference"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="License"></a>
</p>

---

## Why LocalBase?

Need persistent storage without the overhead of setting up PostgreSQL, MongoDB, or even SQLite? **LocalBase** stores each record as an individual JSON file on disk with:

- **Zero external infrastructure** — no database server to install or manage
- **Thread-safe concurrency** — per-collection `sync.RWMutex` for concurrent reads with exclusive writes
- **Atomic writes** — write-to-temp-then-rename prevents partial/corrupt data on crashes
- **Simple API** — CRUD operations, filtered queries, batch writes, and collection management
- **Human-readable storage** — data stored as formatted JSON files you can inspect with any editor

## Architecture

```
┌──────────────────────────────────────────────────────┐
│                    LocalBase Driver                    │
│                                                        │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐ │
│  │   Write()    │  │   Read()     │  │  Where()     │ │
│  │   Update()   │  │   ReadAll()  │  │  Count()     │ │
│  │   BatchWrite │  │              │  │  FieldEquals  │ │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘ │
│         │                 │                  │         │
│  ┌──────▼─────────────────▼──────────────────▼──────┐ │
│  │         Per-Collection sync.RWMutex               │ │
│  │     (concurrent reads, exclusive writes)          │ │
│  └──────────────────────┬───────────────────────────┘ │
│                         │                              │
│  ┌──────────────────────▼───────────────────────────┐ │
│  │              Atomic File I/O Layer                │ │
│  │        (write .tmp → rename to .json)             │ │
│  └──────────────────────┬───────────────────────────┘ │
└─────────────────────────┼────────────────────────────┘
                          │
                   ┌──────▼──────┐
                   │  Filesystem │
                   │             │
                   │ data/       │
                   │ ├── users/  │
                   │ │  ├── alice.json
                   │ │  └── bob.json
                   │ └── orders/ │
                   │    └── order_001.json
                   └─────────────┘
```

## Installation

```bash
go get github.com/ankitsharma97/localbase
```

## Quick Start

```go
package main

import (
    "encoding/json"
    "fmt"
    "log"

    "github.com/ankitsharma97/localbase"
)

type User struct {
    Name    string `json:"name"`
    Age     int    `json:"age"`
    Company string `json:"company"`
}

func main() {
    // Initialize
    db, err := localbase.New("./data", nil)
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    // Create
    db.Write("users", "alice", User{Name: "Alice", Age: 30, Company: "Google"})
    db.Write("users", "bob", User{Name: "Bob", Age: 25, Company: "Meta"})

    // Read
    var user User
    db.Read("users", "alice", &user)
    fmt.Printf("Read: %+v\n", user)

    // Query
    results, _ := db.Where("users", localbase.FieldEquals("company", "Google"))
    fmt.Printf("Found %d Google employees\n", len(results))

    // Update
    db.Update("users", "alice", func(raw []byte) (interface{}, error) {
        var u User
        json.Unmarshal(raw, &u)
        u.Age = 31
        return u, nil
    })

    // Delete
    db.Delete("users", "bob")
}
```

## API Reference

### Core Operations

| Method | Description |
|--------|-------------|
| `New(dir, opts)` | Create a new database instance |
| `Write(collection, key, value)` | Store a record (create or overwrite) |
| `Read(collection, key, &target)` | Read a single record |
| `ReadAll(collection)` | Read all records as raw JSON strings |
| `Update(collection, key, fn)` | Atomic read-modify-write |
| `Delete(collection, key)` | Delete a single record |
| `DeleteAll(collection)` | Delete all records (keep directory) |
| `DropCollection(collection)` | Delete entire collection with directory |
| `Close()` | Cleanup and shutdown |

### Query & Aggregation

| Method | Description |
|--------|-------------|
| `Where(collection, filterFn)` | Filter records with a predicate function |
| `Count(collection, filterFn)` | Count records matching a filter |
| `FieldEquals(field, value)` | Built-in filter: exact field match |
| `FieldContains(field, substr)` | Built-in filter: case-insensitive substring |

### Batch & Management

| Method | Description |
|--------|-------------|
| `BatchWrite(collection, map)` | Write multiple records in one locked operation |
| `Collections()` | List all collection names |
| `Stats(collection)` | Get document count and total size |

### Error Handling

```go
err := db.Read("users", "nonexistent", &user)
if localbase.IsNotFound(err) {
    // Handle missing resource
}
```

| Error Type | When |
|-----------|------|
| `ErrEmptyDir` | Empty directory path to `New()` |
| `ErrEmptyCollection` | Empty collection name |
| `ErrEmptyResource` | Empty resource name |
| `CollectionNotFoundError` | Collection directory doesn't exist |
| `ResourceNotFoundError` | JSON file doesn't exist |

## Concurrency Model

LocalBase uses **per-collection `sync.RWMutex`** locks:

- **Multiple concurrent readers** — `Read()`, `ReadAll()`, `Where()`, `Stats()` acquire read locks
- **Exclusive writers** — `Write()`, `Update()`, `Delete()`, `BatchWrite()` acquire write locks
- **No cross-collection blocking** — operations on different collections never block each other
- **Atomic write guarantee** — data is written to a `.tmp` file first, then renamed

```
Collection "users":     RWMutex → Read ✓  Read ✓  Read ✓  (concurrent)
                                   Write ✗ (blocks until reads complete)

Collection "orders":    RWMutex → Write ✓ (independent, not blocked by "users")
```

## Running Tests

```bash
# Run all tests
make test

# Run with race detector
make test-race

# Generate coverage report
make test-cover

# Run benchmarks
make bench

# Run everything
make all
```

### Test Coverage

The test suite includes **35+ test cases** covering:

- ✅ All CRUD operations with edge cases
- ✅ Concurrent read/write safety (150 goroutines)
- ✅ Concurrent update atomicity verification
- ✅ Query filters (`FieldEquals`, `FieldContains`, custom functions)
- ✅ Batch operations
- ✅ Error types and `IsNotFound` helper
- ✅ Collection management and statistics
- ✅ Empty/missing resource handling

### Benchmarks

```bash
make bench
```

Benchmarks cover: single writes, single reads, ReadAll at 10/100/1K scale, concurrent reads, concurrent writes, batch writes, and filtered queries.

## Project Structure

```
localbase/
├── localbase.go          # Core engine: Driver, CRUD, batch, stats
├── options.go            # Logger interface, Options, CollectionStats
├── errors.go             # Sentinel errors, typed errors, IsNotFound
├── query.go              # Where, Count, FieldEquals, FieldContains
├── localbase_test.go     # Comprehensive test suite (35+ tests)
├── benchmark_test.go     # Performance benchmarks
├── cmd/
│   └── example/
│       └── main.go       # Full-featured demo application
├── .github/
│   └── workflows/
│       └── ci.yml        # CI: multi-version Go, race, coverage, lint
├── go.mod
├── go.sum
├── Makefile              # Dev commands: test, bench, cover, lint
├── README.md
├── LICENSE
└── .gitignore
```

## Design Decisions

| Decision | Rationale |
|----------|-----------|
| **File-per-record** | Simple, human-readable, easy to debug and back up |
| **`sync.RWMutex`** over `sync.Mutex` | Readers don't block each other — critical for read-heavy workloads |
| **Per-collection locks** | Collections are independent; no need for a global lock |
| **Temp-file + rename** | Atomic writes at the OS level; prevents corrupt files on crash |
| **`interface{}` values** | Flexible — any JSON-serializable Go type works |
| **Functional query filters** | Composable, type-safe, no custom query language needed |
| **Typed errors** | `errors.As()` compatible; `IsNotFound()` helper for clean call sites |

## Future Improvements

- [ ] Write-Ahead Log (WAL) for true transaction support
- [ ] Indexing for O(1) lookups on specific fields
- [ ] TTL/expiry for automatic record cleanup
- [ ] Snappy/Gzip compression for large documents
- [ ] Watch/subscribe for real-time change notifications
- [ ] Configurable serialization (MessagePack, CBOR)

## License

[MIT](LICENSE) — Ankit Sharma

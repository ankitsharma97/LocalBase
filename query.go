package localbase

import (
	"encoding/json"
	"fmt"
	"strings"
)

// QueryFunc is a predicate that determines whether a raw JSON record matches
// a query condition. It receives the raw JSON bytes and returns true if the
// record should be included in the results.
type QueryFunc func(raw []byte) bool

// Where reads all records in a collection and returns only those matching
// the provided filter function.
//
// Results are returned as raw JSON strings. Use json.Unmarshal to
// deserialize them into your target types.
//
// If filter is nil, all records are returned (equivalent to ReadAll).
//
// Example:
//
//	results, err := db.Where("users", func(raw []byte) bool {
//	    var user User
//	    json.Unmarshal(raw, &user)
//	    return user.Age > 25
//	})
func (d *Driver) Where(collection string, filter QueryFunc) ([]string, error) {
	if collection == "" {
		return nil, ErrEmptyCollection
	}
	if filter == nil {
		return d.ReadAll(collection)
	}

	all, err := d.ReadAll(collection)
	if err != nil {
		return nil, err
	}

	var matched []string
	for _, record := range all {
		if filter([]byte(record)) {
			matched = append(matched, record)
		}
	}
	return matched, nil
}

// Count returns the number of records in a collection matching the filter.
//
// If filter is nil, returns the total count of all records using the
// more efficient Stats path.
func (d *Driver) Count(collection string, filter QueryFunc) (int, error) {
	if filter == nil {
		stats, err := d.Stats(collection)
		if err != nil {
			return 0, err
		}
		return stats.DocumentCount, nil
	}

	results, err := d.Where(collection, filter)
	if err != nil {
		return 0, err
	}
	return len(results), nil
}

// FieldEquals returns a QueryFunc that matches records where the specified
// top-level JSON field equals the given value.
//
// Comparison is performed using string formatting, which handles strings,
// numbers, and booleans. For nested fields, write a custom QueryFunc.
//
// Example:
//
//	results, _ := db.Where("users", localbase.FieldEquals("city", "Mumbai"))
func FieldEquals(field string, value interface{}) QueryFunc {
	return func(raw []byte) bool {
		var m map[string]interface{}
		if err := json.Unmarshal(raw, &m); err != nil {
			return false
		}
		v, ok := m[field]
		if !ok {
			return false
		}
		return fmt.Sprintf("%v", v) == fmt.Sprintf("%v", value)
	}
}

// FieldContains returns a QueryFunc that matches records where the specified
// string field contains the given substring. The comparison is case-insensitive.
//
// Returns false for non-string fields.
//
// Example:
//
//	results, _ := db.Where("users", localbase.FieldContains("name", "ali"))
func FieldContains(field, substring string) QueryFunc {
	return func(raw []byte) bool {
		var m map[string]interface{}
		if err := json.Unmarshal(raw, &m); err != nil {
			return false
		}
		v, ok := m[field]
		if !ok {
			return false
		}
		str, ok := v.(string)
		if !ok {
			return false
		}
		return strings.Contains(
			strings.ToLower(str),
			strings.ToLower(substring),
		)
	}
}

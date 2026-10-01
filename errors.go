package localbase

import (
	"errors"
	"fmt"
)

// Sentinel errors for common validation failures.
var (
	// ErrEmptyDir is returned when an empty directory path is provided to New.
	ErrEmptyDir = errors.New("localbase: directory path cannot be empty")

	// ErrEmptyCollection is returned when an empty collection name is provided.
	ErrEmptyCollection = errors.New("localbase: collection name cannot be empty")

	// ErrEmptyResource is returned when an empty resource name is provided.
	ErrEmptyResource = errors.New("localbase: resource name cannot be empty")
)

// CollectionNotFoundError is returned when an operation references a
// collection that does not exist on disk.
type CollectionNotFoundError struct {
	Collection string
}

func (e *CollectionNotFoundError) Error() string {
	return fmt.Sprintf("localbase: collection %q not found", e.Collection)
}

// ResourceNotFoundError is returned when an operation references a
// resource that does not exist within a collection.
type ResourceNotFoundError struct {
	Collection string
	Resource   string
}

func (e *ResourceNotFoundError) Error() string {
	return fmt.Sprintf("localbase: resource %q not found in collection %q", e.Resource, e.Collection)
}

// IsNotFound reports whether an error indicates a missing collection or
// resource. This is useful for callers who want to handle "not found"
// conditions without type-asserting to specific error types.
//
// Example:
//
//	err := db.Read("users", "john", &user)
//	if localbase.IsNotFound(err) {
//	    // handle missing resource
//	}
func IsNotFound(err error) bool {
	var cnf *CollectionNotFoundError
	var rnf *ResourceNotFoundError
	return errors.As(err, &cnf) || errors.As(err, &rnf)
}

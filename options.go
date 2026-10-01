package localbase

import "fmt"

// Logger defines the interface for pluggable logging backends.
//
// Any logger that implements these methods can be used with LocalBase,
// including standard library loggers, zerolog, zap, logrus, etc.
//
// The lumber package's ConsoleLogger is used as the default implementation.
type Logger interface {
	Fatal(string, ...interface{})
	Error(string, ...interface{})
	Warn(string, ...interface{})
	Info(string, ...interface{})
	Debug(string, ...interface{})
	Trace(string, ...interface{})
}

// Options configures the behavior of a LocalBase driver instance.
type Options struct {
	// Logger specifies the logger to use for internal log messages.
	// If nil, a default console logger at INFO level is used.
	Logger Logger
}

// CollectionStats holds storage metrics for a single collection.
type CollectionStats struct {
	// Collection is the name of the collection.
	Collection string `json:"collection"`

	// Path is the absolute filesystem path to the collection directory.
	Path string `json:"path"`

	// DocumentCount is the number of JSON documents in the collection.
	DocumentCount int `json:"document_count"`

	// TotalSizeBytes is the combined size of all documents in bytes.
	TotalSizeBytes int64 `json:"total_size_bytes"`
}

// String returns a human-readable representation of collection statistics.
func (s *CollectionStats) String() string {
	return fmt.Sprintf("Collection: %s | Documents: %d | Size: %s",
		s.Collection, s.DocumentCount, formatBytes(s.TotalSizeBytes))
}

// formatBytes converts a byte count to a human-readable string.
func formatBytes(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)
	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

package storage

import (
	"context"
	"io"
	"time"
)

// FileInfo contains metadata about a stored file
type FileInfo struct {
	Path         string    `json:"path"`
	Size         int64     `json:"size"`
	ModifiedTime time.Time `json:"modified_time"`
	IsDir        bool      `json:"is_dir"`
}

// StorageStats contains aggregate storage statistics
type StorageStats struct {
	TotalSize  int64 `json:"total_size"`
	TotalFiles int64 `json:"total_files"`
}

// Storage defines the interface for file storage backends
type Storage interface {
	// Save saves a file to storage and returns the path/key
	Save(ctx context.Context, path string, reader io.Reader) error

	// Get retrieves a file from storage
	Get(ctx context.Context, path string) (io.ReadCloser, error)

	// Delete removes a file from storage
	Delete(ctx context.Context, path string) error

	// Exists checks if a file exists in storage
	Exists(ctx context.Context, path string) (bool, error)

	// GetURL returns a URL to access the file (for S3, returns presigned URL or public URL)
	GetURL(ctx context.Context, path string) (string, error)

	// List lists files in storage with optional prefix filter
	List(ctx context.Context, prefix string) ([]FileInfo, error)

	// Stat returns metadata about a specific file
	Stat(ctx context.Context, path string) (*FileInfo, error)

	// GetStats returns aggregate storage statistics
	GetStats(ctx context.Context) (*StorageStats, error)
}

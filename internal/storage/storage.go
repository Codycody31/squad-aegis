package storage

import (
	"context"
	"io"
)

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
}


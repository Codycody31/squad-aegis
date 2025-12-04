package storage

import (
	"fmt"

	"go.codycody31.dev/squad-aegis/internal/shared/config"
)

// NewStorage creates a storage instance based on configuration
func NewStorage(cfg config.Struct) (Storage, error) {
	switch cfg.Storage.Type {
	case "local":
		basePath := cfg.Storage.Local.Path
		if basePath == "" {
			basePath = "storage"
		}
		return NewLocalStorage(basePath)
	case "s3":
		return NewS3Storage(cfg)
	default:
		return nil, fmt.Errorf("unsupported storage type: %s (supported: local, s3)", cfg.Storage.Type)
	}
}

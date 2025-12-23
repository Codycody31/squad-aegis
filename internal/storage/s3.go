package storage

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/internal/shared/config"
)

// S3Storage implements Storage interface for S3-compatible storage (AWS S3, MinIO, etc.)
type S3Storage struct {
	client   *minio.Client
	bucket   string
	basePath string
}

// NewS3Storage creates a new S3 storage instance using MinIO SDK
func NewS3Storage(cfg config.Struct) (*S3Storage, error) {
	bucketName := cfg.Storage.S3.Bucket
	if bucketName == "" {
		return nil, fmt.Errorf("S3 bucket name is required")
	}

	endpoint := cfg.Storage.S3.Endpoint
	if endpoint == "" {
		// Default to AWS S3 endpoint if not specified
		endpoint = fmt.Sprintf("s3.%s.amazonaws.com", cfg.Storage.S3.Region)
	}

	accessKeyID := cfg.Storage.S3.AccessKeyID
	secretAccessKey := cfg.Storage.S3.SecretAccessKey

	if accessKeyID == "" || secretAccessKey == "" {
		return nil, fmt.Errorf("S3 access key ID and secret access key are required")
	}

	useSSL := cfg.Storage.S3.UseSSL

	// Initialize MinIO client
	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: useSSL,
		Region: cfg.Storage.S3.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create MinIO client: %w", err)
	}

	ctx := context.Background()

	// Check if bucket exists, create if it doesn't
	exists, err := minioClient.BucketExists(ctx, bucketName)
	if err != nil {
		return nil, fmt.Errorf("failed to check bucket existence: %w", err)
	}

	if !exists {
		// Create bucket
		err = minioClient.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{
			Region: cfg.Storage.S3.Region,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create bucket: %w", err)
		}
		log.Info().Str("bucket", bucketName).Msg("Created S3 bucket")
	}

	log.Info().
		Str("bucket", bucketName).
		Str("endpoint", endpoint).
		Str("region", cfg.Storage.S3.Region).
		Msg("Connected to S3 storage")

	return &S3Storage{
		client:   minioClient,
		bucket:   bucketName,
		basePath: "evidence", // Base path within bucket
	}, nil
}

// Save saves a file to S3
func (s *S3Storage) Save(ctx context.Context, path string, reader io.Reader) error {
	key := s.getKey(path)

	_, err := s.client.PutObject(ctx, s.bucket, key, reader, -1, minio.PutObjectOptions{
		ContentType: "application/octet-stream",
	})
	if err != nil {
		return fmt.Errorf("failed to upload to S3: %w", err)
	}

	return nil
}

// Get retrieves a file from S3
func (s *S3Storage) Get(ctx context.Context, path string) (io.ReadCloser, error) {
	key := s.getKey(path)

	object, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get object from S3: %w", err)
	}

	return object, nil
}

// Delete removes a file from S3
func (s *S3Storage) Delete(ctx context.Context, path string) error {
	key := s.getKey(path)

	err := s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete object from S3: %w", err)
	}

	return nil
}

// Exists checks if a file exists in S3
func (s *S3Storage) Exists(ctx context.Context, path string) (bool, error) {
	key := s.getKey(path)

	_, err := s.client.StatObject(ctx, s.bucket, key, minio.StatObjectOptions{})
	if err != nil {
		// Check if error is "not found"
		errResp := minio.ToErrorResponse(err)
		if errResp.Code == "NoSuchKey" || errResp.Code == "NotFound" {
			return false, nil
		}
		return false, fmt.Errorf("failed to check object existence: %w", err)
	}

	return true, nil
}

// GetURL returns a presigned URL for S3 object (valid for 1 hour)
func (s *S3Storage) GetURL(ctx context.Context, path string) (string, error) {
	key := s.getKey(path)

	url, err := s.client.PresignedGetObject(ctx, s.bucket, key, time.Hour, nil)
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}

	return url.String(), nil
}

// getKey returns the full S3 key for a given path
func (s *S3Storage) getKey(path string) string {
	if s.basePath != "" {
		return fmt.Sprintf("%s/%s", s.basePath, path)
	}
	return path
}

// List lists files in S3 storage with optional prefix filter
func (s *S3Storage) List(ctx context.Context, prefix string) ([]FileInfo, error) {
	var files []FileInfo
	searchPrefix := s.getKey(prefix)

	objectCh := s.client.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{
		Prefix:    searchPrefix,
		Recursive: true,
	})

	for object := range objectCh {
		if object.Err != nil {
			return nil, fmt.Errorf("error listing objects: %w", object.Err)
		}

		// Remove base path to get relative path
		relPath := object.Key
		if s.basePath != "" && len(object.Key) > len(s.basePath)+1 {
			relPath = object.Key[len(s.basePath)+1:]
		}

		files = append(files, FileInfo{
			Path:         relPath,
			Size:         object.Size,
			ModifiedTime: object.LastModified,
			IsDir:        false, // S3 doesn't have directories, only keys
		})
	}

	return files, nil
}

// Stat returns metadata about a specific file in S3
func (s *S3Storage) Stat(ctx context.Context, path string) (*FileInfo, error) {
	key := s.getKey(path)

	objInfo, err := s.client.StatObject(ctx, s.bucket, key, minio.StatObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to stat object: %w", err)
	}

	return &FileInfo{
		Path:         path,
		Size:         objInfo.Size,
		ModifiedTime: objInfo.LastModified,
		IsDir:        false,
	}, nil
}

// GetStats returns aggregate storage statistics for S3
func (s *S3Storage) GetStats(ctx context.Context) (*StorageStats, error) {
	stats := &StorageStats{}

	objectCh := s.client.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{
		Prefix:    s.basePath,
		Recursive: true,
	})

	for object := range objectCh {
		if object.Err != nil {
			return nil, fmt.Errorf("error listing objects: %w", object.Err)
		}

		stats.TotalSize += object.Size
		stats.TotalFiles++
	}

	return stats, nil
}


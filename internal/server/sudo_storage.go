package server

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"go.codycody31.dev/squad-aegis/internal/server/responses"
	"go.codycody31.dev/squad-aegis/internal/storage"
)

// StorageFileResponse represents a file in storage
type StorageFileResponse struct {
	Path         string `json:"path"`
	Size         int64  `json:"size"`
	SizeReadable string `json:"size_readable"`
	ModifiedTime string `json:"modified_time"`
	IsDir        bool   `json:"is_dir"`
	Extension    string `json:"extension"`
}

// StorageSummaryResponse represents storage usage statistics
type StorageSummaryResponse struct {
	TotalSize         int64                       `json:"total_size"`
	TotalSizeReadable string                      `json:"total_size_readable"`
	TotalFiles        int64                       `json:"total_files"`
	FilesByType       map[string]int              `json:"files_by_type"`
	SizeByType        map[string]int64            `json:"size_by_type"`
	StorageType       string                      `json:"storage_type"`
	RecentFiles       []StorageFileResponse       `json:"recent_files"`
}

// BulkDeleteRequest represents a bulk delete operation
type BulkDeleteRequest struct {
	Paths []string `json:"paths" binding:"required"`
}

// GetStorageSummary returns storage usage statistics
func (s *Server) GetStorageSummary(c *gin.Context) {
	ctx := c.Request.Context()

	stats, err := s.Dependencies.Storage.GetStats(ctx)
	if err != nil {
		responses.InternalServerError(c, fmt.Errorf("failed to get storage stats: %w", err), nil)
		return
	}

	// Get list of files for categorization
	files, err := s.Dependencies.Storage.List(ctx, "")
	if err != nil {
		responses.InternalServerError(c, fmt.Errorf("failed to list files: %w", err), nil)
		return
	}

	// Categorize files by type
	filesByType := make(map[string]int)
	sizeByType := make(map[string]int64)
	var recentFiles []StorageFileResponse

	for _, file := range files {
		if file.IsDir {
			continue
		}

		ext := strings.ToLower(filepath.Ext(file.Path))
		if ext == "" {
			ext = "no_extension"
		}

		filesByType[ext]++
		sizeByType[ext] += file.Size

		recentFiles = append(recentFiles, StorageFileResponse{
			Path:         file.Path,
			Size:         file.Size,
			SizeReadable: formatBytes(file.Size),
			ModifiedTime: file.ModifiedTime.Format("2006-01-02 15:04:05"),
			IsDir:        file.IsDir,
			Extension:    ext,
		})
	}

	// Sort recent files by modified time and take top 10
	if len(recentFiles) > 10 {
		// Simple sorting by keeping the most recent
		recentFiles = recentFiles[len(recentFiles)-10:]
	}

	// Reverse for newest first
	for i := 0; i < len(recentFiles)/2; i++ {
		j := len(recentFiles) - i - 1
		recentFiles[i], recentFiles[j] = recentFiles[j], recentFiles[i]
	}

	// Determine storage type
	storageType := "local"
	if _, ok := s.Dependencies.Storage.(*storage.S3Storage); ok {
		storageType = "s3"
	}

	summary := StorageSummaryResponse{
		TotalSize:         stats.TotalSize,
		TotalSizeReadable: formatBytes(stats.TotalSize),
		TotalFiles:        stats.TotalFiles,
		FilesByType:       filesByType,
		SizeByType:        sizeByType,
		StorageType:       storageType,
		RecentFiles:       recentFiles,
	}

	responses.Success(c, "Storage summary retrieved successfully", &gin.H{"data": summary})
}

// GetStorageFiles returns a paginated list of files in storage
func (s *Server) GetStorageFiles(c *gin.Context) {
	ctx := c.Request.Context()

	// Get query parameters
	prefix := c.DefaultQuery("prefix", "")
	pageStr := c.DefaultQuery("page", "1")
	limitStr := c.DefaultQuery("limit", "50")

	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 || limit > 100 {
		limit = 50
	}

	// List all files with prefix
	files, err := s.Dependencies.Storage.List(ctx, prefix)
	if err != nil {
		responses.InternalServerError(c, fmt.Errorf("failed to list files: %w", err), nil)
		return
	}

	// Filter out directories if needed and convert to response format
	var fileResponses []StorageFileResponse
	for _, file := range files {
		fileResponses = append(fileResponses, StorageFileResponse{
			Path:         file.Path,
			Size:         file.Size,
			SizeReadable: formatBytes(file.Size),
			ModifiedTime: file.ModifiedTime.Format("2006-01-02 15:04:05"),
			IsDir:        file.IsDir,
			Extension:    strings.ToLower(filepath.Ext(file.Path)),
		})
	}

	// Calculate pagination
	total := len(fileResponses)
	start := (page - 1) * limit
	end := start + limit

	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	paginatedFiles := fileResponses[start:end]

	responses.Success(c, "Files retrieved successfully", &gin.H{
		"files": paginatedFiles,
		"pagination": gin.H{
			"page":       page,
			"limit":      limit,
			"total":      total,
			"totalPages": (total + limit - 1) / limit,
		},
	})
}

// DownloadStorageFile downloads a specific file from storage
func (s *Server) DownloadStorageFile(c *gin.Context) {
	ctx := c.Request.Context()
	path := c.Param("path")

	if path == "" {
		responses.BadRequest(c, "File path is required", nil)
		return
	}

	// Security check: prevent directory traversal
	if strings.Contains(path, "..") {
		responses.BadRequest(c, "Invalid file path", nil)
		return
	}

	// Check if file exists
	exists, err := s.Dependencies.Storage.Exists(ctx, path)
	if err != nil {
		responses.InternalServerError(c, fmt.Errorf("failed to check file existence: %w", err), nil)
		return
	}

	if !exists {
		responses.NotFound(c, "File not found", nil)
		return
	}

	// Get file
	reader, err := s.Dependencies.Storage.Get(ctx, path)
	if err != nil {
		responses.InternalServerError(c, fmt.Errorf("failed to get file: %w", err), nil)
		return
	}
	defer reader.Close()

	// Set headers for download
	filename := filepath.Base(path)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	c.Header("Content-Type", "application/octet-stream")

	// Stream file to client
	c.DataFromReader(200, -1, "application/octet-stream", reader, nil)
}

// DeleteStorageFile deletes a specific file from storage
func (s *Server) DeleteStorageFile(c *gin.Context) {
	ctx := c.Request.Context()
	path := c.Param("path")

	if path == "" {
		responses.BadRequest(c, "File path is required", nil)
		return
	}

	// Security check: prevent directory traversal
	if strings.Contains(path, "..") {
		responses.BadRequest(c, "Invalid file path", nil)
		return
	}

	// Check if file exists
	exists, err := s.Dependencies.Storage.Exists(ctx, path)
	if err != nil {
		responses.InternalServerError(c, fmt.Errorf("failed to check file existence: %w", err), nil)
		return
	}

	if !exists {
		responses.NotFound(c, "File not found", nil)
		return
	}

	// Delete file
	err = s.Dependencies.Storage.Delete(ctx, path)
	if err != nil {
		responses.InternalServerError(c, fmt.Errorf("failed to delete file: %w", err), nil)
		return
	}

	responses.SimpleSuccess(c, "File deleted successfully")
}

// BulkDeleteStorageFiles deletes multiple files from storage
func (s *Server) BulkDeleteStorageFiles(c *gin.Context) {
	ctx := c.Request.Context()

	var req BulkDeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		responses.BadRequest(c, "Invalid request payload", nil)
		return
	}

	if len(req.Paths) == 0 {
		responses.BadRequest(c, "No files specified for deletion", nil)
		return
	}

	// Security check: prevent directory traversal
	for _, path := range req.Paths {
		if strings.Contains(path, "..") {
			responses.BadRequest(c, "Invalid file path", nil)
			return
		}
	}

	// Delete files
	deleted := 0
	failed := 0
	errors := []string{}

	for _, path := range req.Paths {
		err := s.Dependencies.Storage.Delete(ctx, path)
		if err != nil {
			failed++
			errors = append(errors, fmt.Sprintf("%s: %v", path, err))
		} else {
			deleted++
		}
	}

	responses.Success(c, "Bulk delete completed", &gin.H{
		"deleted": deleted,
		"failed":  failed,
		"errors":  errors,
	})
}

// formatBytes converts bytes to human-readable format
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}


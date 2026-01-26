package server

import (
	"fmt"
	"mime"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/internal/core"
	"go.codycody31.dev/squad-aegis/internal/server/responses"
)

const (
	maxFileSize = 50 * 1024 * 1024 // 50MB
)

// previewableTypes defines MIME types that can be displayed inline in the browser
var previewableTypes = map[string]bool{
	"image/jpeg":      true,
	"image/jpg":       true,
	"image/png":       true,
	"image/gif":       true,
	"image/webp":      true,
	"video/mp4":       true,
	"video/webm":      true,
	"video/quicktime": true,
	"application/pdf": true,
	"text/plain":      true,
}

// isPreviewableType checks if a content type can be previewed inline
func isPreviewableType(contentType string) bool {
	return previewableTypes[strings.ToLower(contentType)]
}

// ServerEvidenceFileUpload handles file uploads for ban evidence
func (s *Server) ServerEvidenceFileUpload(c *gin.Context) {
	user := s.getUserFromSession(c)

	serverIdString := c.Param("serverId")
	serverId, err := uuid.Parse(serverIdString)
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	// Check if user has access to this server
	_, err = core.GetServerById(c.Request.Context(), s.Dependencies.DB, serverId, user)
	if err != nil {
		responses.BadRequest(c, "Failed to get server", &gin.H{"error": err.Error()})
		return
	}

	// Get the file from the form
	file, err := c.FormFile("file")
	if err != nil {
		responses.BadRequest(c, "No file provided", &gin.H{"error": err.Error()})
		return
	}

	// Validate file size
	if file.Size > maxFileSize {
		responses.BadRequest(c, "File too large", &gin.H{
			"error":      fmt.Sprintf("File size exceeds maximum of %d bytes", maxFileSize),
			"max_size":   maxFileSize,
			"file_size": file.Size,
		})
		return
	}

	// Validate file type
	allowedTypes := map[string]bool{
		"image/jpeg":      true,
		"image/jpg":       true,
		"image/png":       true,
		"image/gif":       true,
		"image/webp":      true,
		"video/mp4":       true,
		"video/webm":      true,
		"video/quicktime": true,
		"application/pdf": true,
		"text/plain":      true,
	}

	fileType := file.Header.Get("Content-Type")
	if fileType == "" {
		// Try to detect from extension
		ext := filepath.Ext(file.Filename)
		fileType = mime.TypeByExtension(ext)
	}

	if fileType == "" || !allowedTypes[strings.ToLower(fileType)] {
		responses.BadRequest(c, "Invalid file type", &gin.H{
			"error":     "File type not allowed",
			"file_type": fileType,
			"allowed_types": []string{
				"image/jpeg", "image/png", "image/gif", "image/webp",
				"video/mp4", "video/webm", "video/quicktime",
				"application/pdf", "text/plain",
			},
		})
		return
	}

	// Generate unique filename
	fileExt := filepath.Ext(file.Filename)
	fileID := uuid.New()
	fileName := fmt.Sprintf("%s%s", fileID.String(), fileExt)
	
	// Build storage path: evidence/{serverId}/{fileId}.ext
	// The "evidence" prefix ensures files are organized under the storage base path
	storagePath := fmt.Sprintf("evidence/%s/%s", serverId.String(), fileName)

	// Open the uploaded file
	src, err := file.Open()
	if err != nil {
		responses.BadRequest(c, "Failed to open uploaded file", &gin.H{"error": err.Error()})
		return
	}
	defer src.Close()

	// Save the file using storage backend
	if err := s.Dependencies.Storage.Save(c.Request.Context(), storagePath, src); err != nil {
		log.Error().Err(err).Str("path", storagePath).Msg("Failed to save file to storage")
		responses.BadRequest(c, "Failed to save file", &gin.H{"error": err.Error()})
		return
	}

	// Return file information
	responses.Success(c, "File uploaded successfully", &gin.H{
		"file_id":   fileID.String(),
		"file_name": file.Filename,
		"file_path": storagePath,
		"file_size": file.Size,
		"file_type": fileType,
	})
}

// ServerEvidenceFileDownload handles downloading evidence files
func (s *Server) ServerEvidenceFileDownload(c *gin.Context) {
	user := s.getUserFromSession(c)

	serverIdString := c.Param("serverId")
	serverId, err := uuid.Parse(serverIdString)
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	fileId := c.Param("fileId")
	if fileId == "" {
		responses.BadRequest(c, "File ID is required", nil)
		return
	}

	// Check if user has access to this server
	_, err = core.GetServerById(c.Request.Context(), s.Dependencies.DB, serverId, user)
	if err != nil {
		responses.BadRequest(c, "Failed to get server", &gin.H{"error": err.Error()})
		return
	}

	// Verify the file belongs to evidence for this server
	var filePath, fileName string
	err = s.Dependencies.DB.QueryRowContext(c.Request.Context(), `
		SELECT file_path, file_name
		FROM ban_evidence
		WHERE server_id = $1 AND file_path LIKE $2
		LIMIT 1
	`, serverId, "%"+fileId+"%").Scan(&filePath, &fileName)
	if err != nil {
		responses.BadRequest(c, "File not found", &gin.H{"error": "File not found or access denied"})
		return
	}

	// Check if file exists in storage
	exists, err := s.Dependencies.Storage.Exists(c.Request.Context(), filePath)
	if err != nil {
		log.Error().Err(err).Str("path", filePath).Msg("Failed to check file existence")
		responses.BadRequest(c, "Failed to check file", &gin.H{"error": err.Error()})
		return
	}
	if !exists {
		responses.BadRequest(c, "File not found", &gin.H{"error": "File does not exist in storage"})
		return
	}

	// Get file from storage
	fileReader, err := s.Dependencies.Storage.Get(c.Request.Context(), filePath)
	if err != nil {
		log.Error().Err(err).Str("path", filePath).Msg("Failed to get file from storage")
		responses.BadRequest(c, "Failed to retrieve file", &gin.H{"error": err.Error()})
		return
	}
	defer fileReader.Close()

	// Determine content type from file extension
	contentType := mime.TypeByExtension(filepath.Ext(fileName))
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// Check if inline preview is requested
	inline := c.Query("inline") == "true"

	// Set headers based on inline parameter
	if inline && isPreviewableType(contentType) {
		c.Header("Content-Disposition", fmt.Sprintf("inline; filename=%s", fileName))
		c.Header("Content-Type", contentType)
	} else {
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fileName))
		c.Header("Content-Type", "application/octet-stream")
	}

	// Stream file to response
	c.DataFromReader(200, -1, contentType, fileReader, nil)
}


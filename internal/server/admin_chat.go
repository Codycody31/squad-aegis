package server

import (
	"strconv"

	"github.com/gin-gonic/gin"
	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/google/uuid"
	"go.codycody31.dev/squad-aegis/internal/core"
	"go.codycody31.dev/squad-aegis/internal/server/responses"
)

// AdminChatMessageRequest represents a request to create an admin chat message
type AdminChatMessageRequest struct {
	Message string `json:"message" validate:"required"`
}

// AdminChatList handles listing admin chat messages
func (s *Server) AdminChatList(c *gin.Context) {
	// Get pagination parameters
	limitStr := c.DefaultQuery("limit", "50")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 50
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	// Check if we're filtering by server
	var serverId *uuid.UUID
	serverIdStr := c.Query("server_id")
	if serverIdStr != "" {
		serverUUID, err := uuid.Parse(serverIdStr)
		if err == nil {
			serverId = &serverUUID
		}
	}

	// Get messages
	messages, err := core.GetAdminChatMessages(c.Request.Context(), s.Dependencies.DB, serverId, limit, offset)
	if err != nil {
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to fetch admin chat messages"})
		return
	}

	responses.Success(c, "Admin chat messages fetched successfully", &gin.H{"messages": messages})
}

// AdminChatCreate handles creating a new admin chat message
func (s *Server) AdminChatCreate(c *gin.Context) {
	var request AdminChatMessageRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		responses.BadRequest(c, "Invalid request payload", &gin.H{"error": err.Error()})
		return
	}

	// Validate the request
	err := validation.ValidateStruct(&request,
		validation.Field(&request.Message, validation.Required),
	)

	if err != nil {
		responses.BadRequest(c, "Invalid request payload", &gin.H{"errors": err})
		return
	}

	// Get the user from the session
	user := s.getUserFromSession(c)
	if user == nil {
		responses.Unauthorized(c, "Unauthorized", nil)
		return
	}

	// Check if we're associating with a server
	var serverId *uuid.UUID
	serverIdStr := c.Query("server_id")
	if serverIdStr != "" {
		serverUUID, err := uuid.Parse(serverIdStr)
		if err == nil {
			// Verify that the user has access to this server
			server, err := core.GetServerById(c.Request.Context(), s.Dependencies.DB, serverUUID, user)
			if err != nil {
				responses.BadRequest(c, "Invalid server ID", &gin.H{"error": "Invalid server ID"})
				return
			}
			serverId = &server.Id
		}
	}

	// Create the message
	message, err := core.CreateAdminChatMessage(c.Request.Context(), s.Dependencies.DB, user.Id, serverId, request.Message)
	if err != nil {
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to create admin chat message"})
		return
	}

	responses.Success(c, "Admin chat message created successfully", &gin.H{"message": message})
}

// ServerAdminChatList handles listing admin chat messages for a specific server
func (s *Server) ServerAdminChatList(c *gin.Context) {
	// Get the server ID from the URL
	serverId := c.Param("serverId")
	if serverId == "" {
		responses.BadRequest(c, "Server ID is required", &gin.H{"error": "Server ID is required"})
		return
	}

	// Parse UUID
	serverUUID, err := uuid.Parse(serverId)
	if err != nil {
		responses.BadRequest(c, "Invalid server ID format", &gin.H{"error": "Invalid server ID format"})
		return
	}

	// Get pagination parameters
	limitStr := c.DefaultQuery("limit", "50")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 50
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	// Get messages for this server
	messages, err := core.GetAdminChatMessages(c.Request.Context(), s.Dependencies.DB, &serverUUID, limit, offset)
	if err != nil {
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to fetch admin chat messages"})
		return
	}

	responses.Success(c, "Admin chat messages fetched successfully", &gin.H{"messages": messages})
}

// ServerAdminChatCreate handles creating a new admin chat message for a specific server
func (s *Server) ServerAdminChatCreate(c *gin.Context) {
	// Get the server ID from the URL
	serverId := c.Param("serverId")
	if serverId == "" {
		responses.BadRequest(c, "Server ID is required", &gin.H{"error": "Server ID is required"})
		return
	}

	// Parse UUID
	serverUUID, err := uuid.Parse(serverId)
	if err != nil {
		responses.BadRequest(c, "Invalid server ID format", &gin.H{"error": "Invalid server ID format"})
		return
	}

	// Get the user from the session
	user := s.getUserFromSession(c)
	if user == nil {
		responses.Unauthorized(c, "Unauthorized", nil)
		return
	}

	// Verify that the user has access to this server
	server, err := core.GetServerById(c.Request.Context(), s.Dependencies.DB, serverUUID, user)
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": "Invalid server ID"})
		return
	}

	var request AdminChatMessageRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		responses.BadRequest(c, "Invalid request payload", &gin.H{"error": err.Error()})
		return
	}

	// Validate the request
	err = validation.ValidateStruct(&request,
		validation.Field(&request.Message, validation.Required),
	)

	if err != nil {
		responses.BadRequest(c, "Invalid request payload", &gin.H{"errors": err})
		return
	}

	// Create the message
	message, err := core.CreateAdminChatMessage(c.Request.Context(), s.Dependencies.DB, user.Id, &server.Id, request.Message)
	if err != nil {
		responses.InternalServerError(c, err, &gin.H{"error": "Failed to create admin chat message"})
		return
	}

	responses.Success(c, "Admin chat message created successfully", &gin.H{"message": message})
}

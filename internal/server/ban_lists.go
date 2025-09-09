package server

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.codycody31.dev/squad-aegis/internal/core"
	"go.codycody31.dev/squad-aegis/internal/models"
	"go.codycody31.dev/squad-aegis/internal/server/responses"
)

// Ban List Management Handlers

// BanListsList handles listing all ban lists
func (s *Server) BanListsList(c *gin.Context) {
	banLists, err := core.GetBanLists(c.Request.Context(), s.Dependencies.DB)
	if err != nil {
		responses.BadRequest(c, "Failed to get ban lists", &gin.H{"error": err.Error()})
		return
	}

	responses.Success(c, "Ban lists fetched successfully", &gin.H{
		"ban_lists": banLists,
	})
}

// BanListsCreate handles creating a new ban list
func (s *Server) BanListsCreate(c *gin.Context) {
	var request models.BanListCreateRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		responses.BadRequest(c, "Invalid request payload", &gin.H{"error": err.Error()})
		return
	}

	// Validate request
	if request.Name == "" {
		responses.BadRequest(c, "Ban list name is required", &gin.H{"error": "Ban list name is required"})
		return
	}

	banList := &models.BanList{
		ID:          uuid.New(),
		Name:        request.Name,
		Description: request.Description,
		IsRemote:    false, // Default to local ban list
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	createdBanList, err := core.CreateBanList(c.Request.Context(), s.Dependencies.DB, banList)
	if err != nil {
		responses.BadRequest(c, "Failed to create ban list", &gin.H{"error": err.Error()})
		return
	}

	responses.Success(c, "Ban list created successfully", &gin.H{
		"ban_list": createdBanList,
	})
}

// BanListsGet handles getting a specific ban list
func (s *Server) BanListsGet(c *gin.Context) {
	banListIdString := c.Param("banListId")
	banListId, err := uuid.Parse(banListIdString)
	if err != nil {
		responses.BadRequest(c, "Invalid ban list ID", &gin.H{"error": err.Error()})
		return
	}

	banList, err := core.GetBanListById(c.Request.Context(), s.Dependencies.DB, banListId)
	if err != nil {
		responses.BadRequest(c, "Failed to get ban list", &gin.H{"error": err.Error()})
		return
	}

	responses.Success(c, "Ban list fetched successfully", &gin.H{
		"ban_list": banList,
	})
}

// BanListsUpdate handles updating a ban list
func (s *Server) BanListsUpdate(c *gin.Context) {
	banListIdString := c.Param("banListId")
	banListId, err := uuid.Parse(banListIdString)
	if err != nil {
		responses.BadRequest(c, "Invalid ban list ID", &gin.H{"error": err.Error()})
		return
	}

	var request models.BanListUpdateRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		responses.BadRequest(c, "Invalid request payload", &gin.H{"error": err.Error()})
		return
	}

	// Validate request
	if request.Name == "" {
		responses.BadRequest(c, "Ban list name is required", &gin.H{"error": "Ban list name is required"})
		return
	}

	updateData := map[string]interface{}{
		"name":        request.Name,
		"description": request.Description,
	}

	err = core.UpdateBanList(c.Request.Context(), s.Dependencies.DB, banListId, updateData)
	if err != nil {
		responses.BadRequest(c, "Failed to update ban list", &gin.H{"error": err.Error()})
		return
	}

	responses.Success(c, "Ban list updated successfully", nil)
}

// BanListsDelete handles deleting a ban list
func (s *Server) BanListsDelete(c *gin.Context) {
	banListIdString := c.Param("banListId")
	banListId, err := uuid.Parse(banListIdString)
	if err != nil {
		responses.BadRequest(c, "Invalid ban list ID", &gin.H{"error": err.Error()})
		return
	}

	err = core.DeleteBanList(c.Request.Context(), s.Dependencies.DB, banListId)
	if err != nil {
		responses.BadRequest(c, "Failed to delete ban list", &gin.H{"error": err.Error()})
		return
	}

	responses.Success(c, "Ban list deleted successfully", nil)
}

// Server Ban List Subscription Handlers

// ServerBanListSubscriptions handles listing ban list subscriptions for a server
func (s *Server) ServerBanListSubscriptions(c *gin.Context) {
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

	subscriptions, err := core.GetServerBanListSubscriptions(c.Request.Context(), s.Dependencies.DB, serverId)
	if err != nil {
		responses.BadRequest(c, "Failed to get subscriptions", &gin.H{"error": err.Error()})
		return
	}

	responses.Success(c, "Subscriptions fetched successfully", &gin.H{
		"subscriptions": subscriptions,
	})
}

// ServerBanListSubscriptionCreate handles subscribing a server to a ban list
func (s *Server) ServerBanListSubscriptionCreate(c *gin.Context) {
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

	var request models.ServerBanListSubscriptionRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		responses.BadRequest(c, "Invalid request payload", &gin.H{"error": err.Error()})
		return
	}

	banListId, err := uuid.Parse(request.BanListID)
	if err != nil {
		responses.BadRequest(c, "Invalid ban list ID", &gin.H{"error": err.Error()})
		return
	}

	subscription, err := core.CreateServerBanListSubscription(c.Request.Context(), s.Dependencies.DB, serverId, banListId)
	if err != nil {
		responses.BadRequest(c, "Failed to create subscription", &gin.H{"error": err.Error()})
		return
	}

	// Create audit log
	auditData := map[string]interface{}{
		"subscriptionId": subscription.ID.String(),
		"banListId":      banListId.String(),
	}
	s.CreateAuditLog(c.Request.Context(), &serverId, &user.Id, "server:banlist:subscribe", auditData)

	responses.Success(c, "Subscription created successfully", &gin.H{
		"subscription": subscription,
	})
}

// ServerBanListSubscriptionDelete handles unsubscribing a server from a ban list
func (s *Server) ServerBanListSubscriptionDelete(c *gin.Context) {
	user := s.getUserFromSession(c)

	serverIdString := c.Param("serverId")
	serverId, err := uuid.Parse(serverIdString)
	if err != nil {
		responses.BadRequest(c, "Invalid server ID", &gin.H{"error": err.Error()})
		return
	}

	banListIdString := c.Param("banListId")
	banListId, err := uuid.Parse(banListIdString)
	if err != nil {
		responses.BadRequest(c, "Invalid ban list ID", &gin.H{"error": err.Error()})
		return
	}

	// Check if user has access to this server
	_, err = core.GetServerById(c.Request.Context(), s.Dependencies.DB, serverId, user)
	if err != nil {
		responses.BadRequest(c, "Failed to get server", &gin.H{"error": err.Error()})
		return
	}

	err = core.DeleteServerBanListSubscription(c.Request.Context(), s.Dependencies.DB, serverId, banListId)
	if err != nil {
		responses.BadRequest(c, "Failed to delete subscription", &gin.H{"error": err.Error()})
		return
	}

	// Create audit log
	auditData := map[string]interface{}{
		"banListId": banListId.String(),
	}
	s.CreateAuditLog(c.Request.Context(), &serverId, &user.Id, "server:banlist:unsubscribe", auditData)

	responses.Success(c, "Subscription deleted successfully", nil)
}

// Remote Ban Source Handlers

// RemoteBanSourcesList handles listing all remote ban sources
func (s *Server) RemoteBanSourcesList(c *gin.Context) {
	sources, err := core.GetRemoteBanSources(c.Request.Context(), s.Dependencies.DB)
	if err != nil {
		responses.BadRequest(c, "Failed to get remote ban sources", &gin.H{"error": err.Error()})
		return
	}

	responses.Success(c, "Remote ban sources fetched successfully", &gin.H{
		"sources": sources,
	})
}

// RemoteBanSourcesCreate handles creating a new remote ban source
func (s *Server) RemoteBanSourcesCreate(c *gin.Context) {
	var request models.RemoteBanSourceCreateRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		responses.BadRequest(c, "Invalid request payload", &gin.H{"error": err.Error()})
		return
	}

	// Validate request
	if request.Name == "" {
		responses.BadRequest(c, "Source name is required", &gin.H{"error": "Source name is required"})
		return
	}
	if request.URL == "" {
		responses.BadRequest(c, "Source URL is required", &gin.H{"error": "Source URL is required"})
		return
	}

	source := &models.RemoteBanSource{
		ID:                  uuid.New(),
		Name:                request.Name,
		URL:                 request.URL,
		SyncEnabled:         request.SyncEnabled,
		SyncIntervalMinutes: request.SyncIntervalMinutes,
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}

	createdSource, err := core.CreateRemoteBanSource(c.Request.Context(), s.Dependencies.DB, source)
	if err != nil {
		responses.BadRequest(c, "Failed to create remote ban source", &gin.H{"error": err.Error()})
		return
	}

	responses.Success(c, "Remote ban source created successfully", &gin.H{
		"source": createdSource,
	})
}

// RemoteBanSourcesUpdate handles updating a remote ban source
func (s *Server) RemoteBanSourcesUpdate(c *gin.Context) {
	sourceIdString := c.Param("sourceId")
	sourceId, err := uuid.Parse(sourceIdString)
	if err != nil {
		responses.BadRequest(c, "Invalid source ID", &gin.H{"error": err.Error()})
		return
	}

	var request models.RemoteBanSourceUpdateRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		responses.BadRequest(c, "Invalid request payload", &gin.H{"error": err.Error()})
		return
	}

	// Validate request
	if request.Name == "" {
		responses.BadRequest(c, "Source name is required", &gin.H{"error": "Source name is required"})
		return
	}
	if request.URL == "" {
		responses.BadRequest(c, "Source URL is required", &gin.H{"error": "Source URL is required"})
		return
	}

	updateData := map[string]interface{}{
		"name":                  request.Name,
		"url":                   request.URL,
		"sync_enabled":          request.SyncEnabled,
		"sync_interval_minutes": request.SyncIntervalMinutes,
	}

	err = core.UpdateRemoteBanSource(c.Request.Context(), s.Dependencies.DB, sourceId, updateData)
	if err != nil {
		responses.BadRequest(c, "Failed to update remote ban source", &gin.H{"error": err.Error()})
		return
	}

	responses.Success(c, "Remote ban source updated successfully", nil)
}

// RemoteBanSourcesDelete handles deleting a remote ban source
func (s *Server) RemoteBanSourcesDelete(c *gin.Context) {
	sourceIdString := c.Param("sourceId")
	sourceId, err := uuid.Parse(sourceIdString)
	if err != nil {
		responses.BadRequest(c, "Invalid source ID", &gin.H{"error": err.Error()})
		return
	}

	err = core.DeleteRemoteBanSource(c.Request.Context(), s.Dependencies.DB, sourceId)
	if err != nil {
		responses.BadRequest(c, "Failed to delete remote ban source", &gin.H{"error": err.Error()})
		return
	}

	responses.Success(c, "Remote ban source deleted successfully", nil)
}

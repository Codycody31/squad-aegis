package server

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.codycody31.dev/squad-aegis/internal/core"
	"go.codycody31.dev/squad-aegis/internal/models"
	"go.codycody31.dev/squad-aegis/internal/server/responses"
)

// IgnoredSteamIDsList returns all ignored Steam IDs
func (s *Server) IgnoredSteamIDsList(c *gin.Context) {
	ignoredSteamIDs, err := core.GetIgnoredSteamIDs(c.Request.Context(), s.Dependencies.DB)
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}

	responses.Success(c, "Ignored Steam IDs retrieved successfully", &gin.H{"ignored_steam_ids": ignoredSteamIDs})
}

// IgnoredSteamIDsCreate creates a new ignored Steam ID entry
func (s *Server) IgnoredSteamIDsCreate(c *gin.Context) {
	var req models.IgnoredSteamIDCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		responses.BadRequest(c, "Invalid request payload", nil)
		return
	}

	user := s.getUserFromSession(c)

	ignoredSteamID := &models.IgnoredSteamID{
		ID:        uuid.New().String(),
		SteamID:   req.SteamID,
		Reason:    req.Reason,
		CreatedBy: &user.Username,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	createdIgnoredSteamID, err := core.CreateIgnoredSteamID(c.Request.Context(), s.Dependencies.DB, ignoredSteamID)
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":          "Ignored Steam ID created successfully",
		"code":             http.StatusCreated,
		"ignored_steam_id": createdIgnoredSteamID,
	})
}

// IgnoredSteamIDsUpdate updates an existing ignored Steam ID entry
func (s *Server) IgnoredSteamIDsUpdate(c *gin.Context) {
	idParam := c.Param("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		responses.BadRequest(c, "Invalid ID format", nil)
		return
	}

	var req models.IgnoredSteamIDUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		responses.BadRequest(c, "Invalid request payload", nil)
		return
	}

	err = core.UpdateIgnoredSteamID(c.Request.Context(), s.Dependencies.DB, id, req)
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}

	// Get the updated entry to return
	updatedIgnoredSteamID, err := core.GetIgnoredSteamIDByID(c.Request.Context(), s.Dependencies.DB, id)
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}

	responses.Success(c, "Ignored Steam ID updated successfully", &gin.H{"ignored_steam_id": updatedIgnoredSteamID})
}

// IgnoredSteamIDsDelete deletes an ignored Steam ID entry
func (s *Server) IgnoredSteamIDsDelete(c *gin.Context) {
	idParam := c.Param("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		responses.BadRequest(c, "Invalid ID format", nil)
		return
	}

	err = core.DeleteIgnoredSteamID(c.Request.Context(), s.Dependencies.DB, id)
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

// IgnoredSteamIDsCheck checks if a Steam ID is in the ignore list
func (s *Server) IgnoredSteamIDsCheck(c *gin.Context) {
	steamID := c.Param("steam_id")
	if steamID == "" {
		responses.BadRequest(c, "Steam ID is required", nil)
		return
	}

	isIgnored, err := core.IsIgnoredSteamID(c.Request.Context(), s.Dependencies.DB, steamID)
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}

	responses.Success(c, "Steam ID check completed", &gin.H{"steam_id": steamID, "is_ignored": isIgnored})
}

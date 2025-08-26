package server

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/leighmacdonald/steamid/v3/steamid"
	"go.codycody31.dev/squad-aegis/internal/core"
	"go.codycody31.dev/squad-aegis/internal/models"
	"go.codycody31.dev/squad-aegis/internal/server/responses"
)

type UserCreateRequest struct {
	SteamId    string `json:"steam_id" binding:"required"`
	Name       string `json:"name" binding:"required"`
	Username   string `json:"username" binding:"required"`
	Password   string `json:"password" binding:"required"`
	SuperAdmin bool   `json:"super_admin"`
}

type UserUpdateRequest struct {
	SteamId    string `json:"steam_id"`
	Name       string `json:"name" binding:"required"`
	SuperAdmin bool   `json:"super_admin"`
}

func (s *Server) UsersList(c *gin.Context) {
	users, err := core.GetUsers(c.Request.Context(), s.Dependencies.DB)
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}

	responses.Success(c, "Users fetched successfully", &gin.H{"users": users})
}

func (s *Server) UserCreate(c *gin.Context) {
	var request UserCreateRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		responses.BadRequest(c, "Invalid request payload", &gin.H{"error": err.Error()})
		return
	}

	sid64 := steamid.New(request.SteamId)
	if !sid64.Valid() {
		responses.BadRequest(c, "Invalid Steam ID", nil)
		return
	}

	userToCreate := models.User{
		SteamId:    sid64.Int64(),
		Name:       request.Name,
		Username:   request.Username,
		Password:   request.Password,
		SuperAdmin: request.SuperAdmin,
	}

	userToCreate.Id = uuid.New()
	userToCreate.CreatedAt = time.Now()
	userToCreate.UpdatedAt = time.Now()

	user, err := core.RegisterUser(c.Request.Context(), s.Dependencies.DB, &userToCreate)
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}

	responses.Success(c, "User created successfully", &gin.H{"user": user})
}

func (s *Server) UserUpdate(c *gin.Context) {
	currentUser := s.getUserFromSession(c)

	// Only super admins can update other users
	if !currentUser.SuperAdmin {
		responses.Forbidden(c, "Only super admins can update users", nil)
		return
	}

	var request UserUpdateRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		responses.BadRequest(c, "Invalid request payload", &gin.H{"error": err.Error()})
		return
	}

	userId, err := uuid.Parse(c.Param("userId"))
	if err != nil {
		responses.BadRequest(c, "Invalid user ID", &gin.H{"error": err.Error()})
		return
	}

	user, err := core.GetUserById(c.Request.Context(), s.Dependencies.DB, userId, nil)
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}

	// Prevent users from removing their own super admin status
	if user.Id == currentUser.Id && currentUser.SuperAdmin && !request.SuperAdmin {
		responses.BadRequest(c, "Cannot remove your own super admin status", nil)
		return
	}

	sid64 := steamid.New(request.SteamId)
	if !sid64.Valid() {
		responses.BadRequest(c, "Invalid Steam ID", nil)
		return
	}

	// Update the user profile using the existing core function
	err = core.UpdateUserProfile(c.Request.Context(), s.Dependencies.DB, userId, request.Name, int(sid64.Int64()))
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}

	// Update super admin status separately if it changed
	if user.SuperAdmin != request.SuperAdmin {
		user.SuperAdmin = request.SuperAdmin
		_, err = core.UpdateUser(c.Request.Context(), s.Dependencies.DB, user)
		if err != nil {
			responses.InternalServerError(c, err, nil)
			return
		}
	}

	// Get updated user data
	updatedUser, err := core.GetUserById(c.Request.Context(), s.Dependencies.DB, userId, nil)
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}

	responses.Success(c, "User updated successfully", &gin.H{"user": updatedUser})
}

func (s *Server) UserDelete(c *gin.Context) {
	currentUser := s.getUserFromSession(c)

	userId := c.Param("userId")

	user, err := core.GetUserById(c.Request.Context(), s.Dependencies.DB, uuid.MustParse(userId), nil)
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}

	if user.Id == currentUser.Id {
		responses.BadRequest(c, "Cannot delete own account", nil)
		return
	}

	err = core.DeleteUser(c.Request.Context(), s.Dependencies.DB, user.Id)
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}

	responses.Success(c, "User deleted successfully", nil)
}

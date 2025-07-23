package server

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.codycody31.dev/squad-aegis/internal/core"
	"go.codycody31.dev/squad-aegis/internal/models"
	"go.codycody31.dev/squad-aegis/internal/server/responses"
)

type UserCreateRequest struct {
	SteamId    int    `json:"steam_id" binding:"required"`
	Name       string `json:"name" binding:"required"`
	Username   string `json:"username" binding:"required"`
	Password   string `json:"password" binding:"required"`
	SuperAdmin bool   `json:"super_admin"`
}

type UserUpdateRequest struct {
	SteamId    int    `json:"steam_id" binding:"required"`
	Name       string `json:"name" binding:"required"`
	Username   string `json:"username" binding:"required"`
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

	userToCreate := models.User{
		SteamId:    request.SteamId,
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
	var request UserUpdateRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		responses.BadRequest(c, "Invalid request payload", &gin.H{"error": err.Error()})
		return
	}

	user, err := core.GetUserById(c.Request.Context(), s.Dependencies.DB, uuid.MustParse(c.Param("userId")), nil)
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}

	user.Name = request.Name
	user.Username = request.Username
	user.SuperAdmin = request.SuperAdmin

	user, err = core.UpdateUser(c.Request.Context(), s.Dependencies.DB, user)
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}

	responses.Success(c, "User updated successfully", &gin.H{"user": user})
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

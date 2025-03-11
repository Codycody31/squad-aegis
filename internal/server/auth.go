package server

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"go.codycody31.dev/squad-aegis/core"
	"go.codycody31.dev/squad-aegis/internal/models"
	"go.codycody31.dev/squad-aegis/internal/server/responses"
)

type AuthLoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type UpdateProfileRequest struct {
	Name    string `json:"name" binding:"required"`
	SteamId *int64 `json:"steamId"`
}

type UpdatePasswordRequest struct {
	CurrentPassword string `json:"currentPassword" binding:"required"`
	NewPassword     string `json:"newPassword" binding:"required,min=8"`
}

func (s *Server) AuthLogin(c *gin.Context) {
	var req AuthLoginRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		responses.BadRequest(c, "Invalid request payload", nil)
		return
	}

	tx, err := s.Dependencies.DB.BeginTx(c.Copy(), nil)
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}

	user, err := core.AuthenticateUser(c.Copy(), tx, req.Username, req.Password)
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}

	session, err := core.CreateSession(c.Copy(), tx, user.Id, c.ClientIP(), time.Hour*24)
	if err != nil {
		fmt.Println(err)
		err := tx.Rollback()
		if err != nil {
			responses.InternalServerError(c, fmt.Errorf("failed to rollback transaction: %w", err), nil)
			return
		}
		responses.InternalServerError(c, err, nil)
		return
	}

	err = tx.Commit()
	if err != nil {
		fmt.Println(err)
		responses.InternalServerError(c, err, nil)
		return
	}

	responses.Success(c, "User logged in successfully", &gin.H{
		"session": gin.H{
			"token":      session.Token,
			"expires_at": session.ExpiresAt,
		},
	})
}

func (s *Server) AuthLogout(c *gin.Context) {
	session := c.MustGet("session").(*models.Session)

	tx, err := s.Dependencies.DB.BeginTx(c.Copy(), nil)
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}

	err = core.DeleteSessionById(c.Copy(), tx, session.Id)
	if err != nil {
		err := tx.Rollback()
		if err != nil {
			responses.InternalServerError(c, fmt.Errorf("failed to rollback transaction: %w", err), nil)
			return
		}
		responses.InternalServerError(c, err, nil)
		return
	}

	err = tx.Commit()
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}

	responses.SimpleSuccess(c, "User logged out")
}

func (s *Server) AuthInitial(c *gin.Context) {
	session := c.MustGet("session").(*models.Session)

	user, err := core.GetUserById(c.Copy(), s.Dependencies.DB, session.UserId, &session.UserId)
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}

	responses.Success(c, "User authenticated", &gin.H{
		"user": user,
	})
}

func (s *Server) UpdateUserProfile(c *gin.Context) {
	session := c.MustGet("session").(*models.Session)
	var req UpdateProfileRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		responses.BadRequest(c, "Invalid request payload", nil)
		return
	}

	tx, err := s.Dependencies.DB.BeginTx(c.Copy(), nil)
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}
	defer tx.Rollback()

	err = core.UpdateUserProfile(c.Copy(), tx, session.UserId, req.Name, req.SteamId)
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}

	if err := tx.Commit(); err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}

	responses.SimpleSuccess(c, "Profile updated successfully")
}

func (s *Server) UpdateUserPassword(c *gin.Context) {
	session := c.MustGet("session").(*models.Session)
	var req UpdatePasswordRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		responses.BadRequest(c, "Invalid request payload", nil)
		return
	}

	tx, err := s.Dependencies.DB.BeginTx(c.Copy(), nil)
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}
	defer tx.Rollback()

	// First verify the current password
	user, err := core.GetUserById(c.Copy(), tx, session.UserId, &session.UserId)
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}

	if err := user.ComparePassword(req.CurrentPassword); err != nil {
		responses.BadRequest(c, "Current password is incorrect", nil)
		return
	}

	// Update the password
	err = core.UpdateUserPassword(c.Copy(), tx, session.UserId, req.NewPassword)
	if err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}

	if err := tx.Commit(); err != nil {
		responses.InternalServerError(c, err, nil)
		return
	}

	responses.SimpleSuccess(c, "Password updated successfully")
}

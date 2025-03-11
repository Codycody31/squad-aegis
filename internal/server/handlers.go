package server

import (
	"fmt"
	"net/http"
	"time"

	"go.codycody31.dev/squad-aegis/core"

	"github.com/gin-gonic/gin"
	"go.codycody31.dev/squad-aegis/internal/models"
	"go.codycody31.dev/squad-aegis/version"
)

func (s *Server) apiHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "Squad Aegis API",
		"data": gin.H{
			"version": version.String(),
		},
	})
}

func (s *Server) healthHandler(c *gin.Context) {
	if err := s.Dependencies.DB.Ping(); err != nil {
		fmt.Println("DB Ping Error: ", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "unhealthy",
			"code":    http.StatusInternalServerError,
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "healthy",
		"code":    http.StatusOK,
	})
}

func (s *Server) customRecovery(c *gin.Context, err any) {
	c.JSON(http.StatusInternalServerError, gin.H{
		"message": "An internal server error occurred",
		"code":    http.StatusInternalServerError,
	})
	c.Abort()
}

func (s *Server) customLoggerWithFormatter(param gin.LogFormatterParams) string {
	return fmt.Sprintf("%s - [%s] \"%s %s %s %d %s \"%s\" %s\"\n",
		param.ClientIP,
		param.TimeStamp.Format(time.RFC1123),
		param.Method,
		param.Path,
		param.Request.Proto,
		param.StatusCode,
		param.Latency,
		param.Request.UserAgent(),
		param.ErrorMessage,
	)
}

func (s *Server) customUserLastSeen(c *gin.Context) {
	session, exists := c.Get("session")
	if exists {
		session := session.(*models.Session)
		_, err := s.Dependencies.DB.Exec("UPDATE sessions SET last_seen = NOW(), last_seen_ip = $1 WHERE id = $2", c.ClientIP(), session.Id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "Internal Server Error",
				"code":    http.StatusInternalServerError,
			})
			c.Abort()
			return
		}
	}
}

func (s *Server) getUserFromSession(c *gin.Context) *models.User {
	session, exists := c.Get("session")
	if exists {
		session := session.(*models.Session)
		user, err := core.GetUserById(c.Copy(), s.Dependencies.DB, session.UserId, &session.UserId)
		if err != nil {
			return nil
		}
		return user
	}
	return nil
}

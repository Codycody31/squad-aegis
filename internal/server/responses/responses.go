package responses

import (
	"net/http"

	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/shared/config"

	"github.com/gin-gonic/gin"
)

// Error sends a JSON response with the given status code, message, and optional data
func Error(c *gin.Context, statusCode int, message string, data *gin.H) {
	response := gin.H{
		"message": message,
		"code":    statusCode,
	}
	if data != nil {
		response["data"] = *data
	}
	c.JSON(statusCode, response)
	c.Abort()
}

// BadRequest sends a 400 Bad Request response
func BadRequest(c *gin.Context, message string, data *gin.H) {
	Error(c, http.StatusBadRequest, message, data)
}

// Unauthorized sends a 401 Unauthorized response
func Unauthorized(c *gin.Context, message string, data *gin.H) {
	Error(c, http.StatusUnauthorized, message, data)
}

// Forbidden sends a 403 Forbidden response
func Forbidden(c *gin.Context, message string, data *gin.H) {
	Error(c, http.StatusForbidden, message, data)
}

// NotFound sends a 404 Not Found response
func NotFound(c *gin.Context, message string, data *gin.H) {
	Error(c, http.StatusNotFound, message, data)
}

// Conflict sends a 409 Conflict response
func Conflict(c *gin.Context, message string, data *gin.H) {
	Error(c, http.StatusConflict, message, data)
}

// InternalServerError sends a 500 Internal Server Error response
func InternalServerError(c *gin.Context, err error, data *gin.H) {
	// TODO: Log the error to Exceptionless and the console
	log.Error().Err(err).Msg("Internal Server Error")

	if config.Config.App.IsDevelopment {
		Error(c, http.StatusInternalServerError, err.Error(), nil)
	} else {
		Error(c, http.StatusInternalServerError, "Internal Server Error", data)
	}
}

// TooManyRequests sends a 429 Too Many Requests response
func TooManyRequests(c *gin.Context, message string, data *gin.H) {
	Error(c, http.StatusTooManyRequests, message, data)
}

// Success sends a 200 OK response with data
func Success(c *gin.Context, message string, data *gin.H) {
	response := gin.H{
		"message": message,
		"code":    http.StatusOK,
	}
	if data != nil {
		response["data"] = *data
	}
	c.JSON(http.StatusOK, response)
}

// SimpleSuccess sends a 200 OK response without data
func SimpleSuccess(c *gin.Context, message string) {
	c.JSON(http.StatusOK, gin.H{
		"message": message,
		"code":    http.StatusOK,
	})
}

// StandardResponse is a standard response format
// @Description Standard response format
type StandardResponse struct {
	Message string      `json:"message"`
	Code    int         `json:"code"`
	Data    interface{} `json:"data"`
}

// SimpleResponse is a simple response format
// @Description Simple response format
type SimpleResponse struct {
	Message string `json:"message"`
	Code    int    `json:"code"`
}

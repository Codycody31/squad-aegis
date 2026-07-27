package server

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"go.codycody31.dev/squad-aegis/internal/server/responses"
)

func parseCursorLimit(c *gin.Context, defaultLimit, maxLimit int) (int, bool) {
	raw := strings.TrimSpace(c.Query("limit"))
	if raw == "" {
		return defaultLimit, true
	}
	limit, err := strconv.Atoi(raw)
	if err != nil || limit < 1 || limit > maxLimit {
		responses.BadRequest(c, "Invalid limit", &gin.H{
			"error": "limit must be between 1 and " + strconv.Itoa(maxLimit),
		})
		return 0, false
	}
	return limit, true
}

func rejectLegacyPagination(c *gin.Context, names ...string) bool {
	for _, name := range names {
		if _, present := c.GetQuery(name); present {
			responses.BadRequest(c, "Invalid pagination parameters", &gin.H{
				"error": name + " is no longer supported; use cursor",
			})
			return true
		}
	}
	return false
}

func normalizedAllFilter(value string) string {
	value = strings.TrimSpace(value)
	if strings.EqualFold(value, "all") {
		return ""
	}
	return value
}

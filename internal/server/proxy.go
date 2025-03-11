package server

import (
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/gin-gonic/gin"
	"go.codycody31.dev/squad-aegis/shared/config"
)

func (s *Server) proxyHandler(c *gin.Context) {
	// Proxy the request to the web ui
	webUiProxy := config.Config.App.WebUiProxy
	remote, err := url.Parse(webUiProxy)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error parsing proxy URL"})
		c.Abort()
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(remote)
	proxy.Director = func(req *http.Request) {
		req.Header = c.Request.Header
		req.Host = remote.Host
		req.URL.Scheme = remote.Scheme
		req.URL.Host = remote.Host
		req.URL.Path = c.Param("proxyPath")
	}

	proxy.ServeHTTP(c.Writer, c.Request)
}

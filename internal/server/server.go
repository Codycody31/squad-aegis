package server

import (
	"database/sql"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"go.codycody31.dev/squad-aegis/internal/connector_manager"
	"go.codycody31.dev/squad-aegis/internal/extension_manager"
	"go.codycody31.dev/squad-aegis/internal/rcon_manager"
	"go.codycody31.dev/squad-aegis/shared/config"

	"github.com/gin-gonic/gin"
)

type Server struct {
	Dependencies *Dependencies
}

type Dependencies struct {
	DB               *sql.DB
	RconManager      *rcon_manager.RconManager
	ConnectorManager *connector_manager.ConnectorManager
	ExtensionManager *extension_manager.ExtensionManager
}

func NewRouter(serverDependencies *Dependencies) *gin.Engine {
	router := gin.New()
	server := &Server{
		Dependencies: serverDependencies,
	}

	if config.Config.Log.ShowGin {
		// General Middleware
		router.Use(gin.Logger())
		router.Use(gin.LoggerWithFormatter(server.customLoggerWithFormatter))
	}

	// Recovery middleware
	router.Use(gin.CustomRecovery(server.customRecovery))

	router.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", config.Config.App.Url)
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, PATCH, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	// Setup user last seen for session
	router.Use(server.customUserLastSeen)

	// Setup the no route handler
	router.NoRoute(gin.WrapF(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api") {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}

		origin, _ := url.Parse(config.Config.App.WebUiProxy)

		director := func(req *http.Request) {
			req.Header.Add("X-Forwarded-Host", req.Host)
			req.Header.Add("X-Origin-Host", origin.Host)
			req.URL.Scheme = origin.Scheme
			req.URL.Host = origin.Host
		}

		proxy := &httputil.ReverseProxy{Director: director}
		proxy.ServeHTTP(w, r)
	}))

	// Setup route group for the API
	apiGroup := router.Group("/api")
	{
		apiGroup.GET("", server.apiHandler)
		apiGroup.GET("/", server.apiHandler)
		apiGroup.GET("/health", server.healthHandler)

		apiGroup.Use(server.OptionalAuthSession)

		authGroup := apiGroup.Group("/auth")
		{
			authGroup.GET("/initial", server.AuthSession, server.AuthInitial)
			authGroup.PATCH("/me", server.AuthSession, server.UpdateUserProfile)
			authGroup.PATCH("/me/password", server.AuthSession, server.UpdateUserPassword)
			authGroup.POST("/logout", server.AuthSession, server.AuthLogout)

			authGroup.Use(func(c *gin.Context) {
				if IsLoggedIn(c) {
					c.JSON(http.StatusUnauthorized, gin.H{
						"message": "Already logged in",
						"code":    http.StatusUnauthorized,
					})
					c.Abort()
					return
				}
			})
			authGroup.POST("/login", server.AuthLogin)
		}

		usersGroup := apiGroup.Group("/users")
		{
			usersGroup.Use(server.AuthSession)
			usersGroup.Use(server.AuthIsSuperAdmin())

			usersGroup.GET("", server.UsersList)
			usersGroup.POST("", server.UserCreate)
			// usersGroup.PUT("/:userId", server.UserUpdate)
			usersGroup.DELETE("/:userId", server.UserDelete)
		}

		serversGroup := apiGroup.Group("/servers")
		{
			serversGroup.Use(server.AuthSession)

			serversGroup.GET("", server.ServersList)
			serversGroup.POST("", server.AuthIsSuperAdmin(), server.ServersCreate)
			serversGroup.GET("/user-roles", server.ServerUserRoles)

			serverGroup := serversGroup.Group("/:serverId")
			{
				serverGroup.GET("", server.ServerGet)
				// serverGroup.PUT("", server.ServerUpdate)
				serverGroup.DELETE("", server.AuthIsSuperAdmin(), server.ServerDelete)

				serverGroup.GET("/status", server.ServerStatus)
				serverGroup.GET("/audit-logs", server.AuthHasServerPermission("manageserver"), server.ServerAuditLogs)

				serverGroup.GET("/rcon/commands", server.RconCommandList)
				serverGroup.GET("/rcon/commands/autocomplete", server.RconCommandAutocomplete)
				serverGroup.POST("/rcon/execute", server.AuthHasServerPermission("manageserver"), server.ServerRconExecute)
				serverGroup.GET("/rcon/server-population", server.ServerRconServerPopulation)
				serverGroup.GET("/rcon/available-layers", server.ServerRconAvailableLayers)
				serverGroup.GET("/rcon/events", server.AuthHasServerPermission("manageserver"), server.ServerRconEvents)

				serverGroup.GET("/roles", server.ServerRolesList)
				serverGroup.POST("/roles", server.AuthIsSuperAdmin(), server.ServerRolesAdd)
				serverGroup.DELETE("/roles/:roleId", server.AuthIsSuperAdmin(), server.ServerRolesRemove)

				serverGroup.GET("/admins", server.ServerAdminsList)
				serverGroup.POST("/admins", server.AuthIsSuperAdmin(), server.ServerAdminsAdd)
				serverGroup.DELETE("/admins/:adminId", server.AuthIsSuperAdmin(), server.ServerAdminsRemove)

				serverGroup.GET("/bans", server.ServerBansList)
				serverGroup.POST("/bans", server.AuthHasAnyServerPermission("ban"), server.ServerBansAdd)
				serverGroup.DELETE("/bans/:banId", server.AuthHasAnyServerPermission("ban"), server.ServerBansRemove)

				// Player action endpoints
				serverGroup.POST("/rcon/kick-player", server.AuthHasAnyServerPermission("kick"), server.ServerRconKickPlayer)
				serverGroup.POST("/rcon/warn-player", server.AuthHasAnyServerPermission("kick"), server.ServerRconWarnPlayer)
				serverGroup.POST("/rcon/move-player", server.AuthHasAnyServerPermission("forceteamchange"), server.ServerRconMovePlayer)

				// Server info endpoints
				serverGroup.GET("/rcon/server-info", server.ServerRconServerInfo)
			}
		}

		// Public Routes for the server
		apiGroup.GET("/servers/:serverId/admins/cfg", server.ServerAdminsCfg)
		apiGroup.GET("/servers/:serverId/bans/cfg", server.ServerBansCfg)

		// Connector management endpoints
		connectorGroup := apiGroup.Group("/connectors")
		{
			connectorGroup.Use(server.AuthSession)
			connectorGroup.Use(server.AuthIsSuperAdmin())

			connectorGroup.GET("/types", server.ListConnectorTypes)
			connectorGroup.GET("/global", server.ListGlobalConnectors)
			connectorGroup.POST("/global", server.CreateGlobalConnector)
			connectorGroup.GET("/global/:id", server.GetGlobalConnector)
			connectorGroup.PUT("/global/:id", server.UpdateGlobalConnector)
			connectorGroup.DELETE("/global/:id", server.DeleteGlobalConnector)
		}

		// Extension management endpoints
		extensionGroup := apiGroup.Group("/extensions")
		{
			extensionGroup.Use(server.AuthSession)

			extensionGroup.GET("/types", server.ListExtensionTypes)
		}

		// Server-specific extension management endpoints
		serversGroup.GET("/:serverId/extensions", server.AuthHasServerPermission("manageserver"), server.ListServerExtensions)
		serversGroup.POST("/:serverId/extensions", server.AuthHasServerPermission("manageserver"), server.CreateServerExtension)
		serversGroup.GET("/:serverId/extensions/:extensionId", server.AuthHasServerPermission("manageserver"), server.GetServerExtension)
		serversGroup.PUT("/:serverId/extensions/:extensionId", server.AuthHasServerPermission("manageserver"), server.UpdateServerExtension)
		serversGroup.DELETE("/:serverId/extensions/:extensionId", server.AuthHasServerPermission("manageserver"), server.DeleteServerExtension)
		serversGroup.POST("/:serverId/extensions/:extensionId/toggle", server.AuthHasServerPermission("manageserver"), server.ToggleServerExtension)
	}

	return router
}

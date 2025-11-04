package server

import (
	"database/sql"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"go.codycody31.dev/squad-aegis/internal/clickhouse"
	"go.codycody31.dev/squad-aegis/internal/core"
	"go.codycody31.dev/squad-aegis/internal/event_manager"
	"go.codycody31.dev/squad-aegis/internal/logwatcher_manager"
	"go.codycody31.dev/squad-aegis/internal/plugin_manager"
	"go.codycody31.dev/squad-aegis/internal/rcon_manager"
	"go.codycody31.dev/squad-aegis/internal/server/web"
	"go.codycody31.dev/squad-aegis/internal/shared/config"
	"go.codycody31.dev/squad-aegis/internal/workflow_manager"

	"github.com/gin-gonic/gin"
)

type Server struct {
	Dependencies *Dependencies
}

type Dependencies struct {
	DB                   *sql.DB
	Clickhouse           *clickhouse.Client
	RconManager          *rcon_manager.RconManager
	EventManager         *event_manager.EventManager
	LogwatcherManager    *logwatcher_manager.LogwatcherManager
	PluginManager        *plugin_manager.PluginManager
	WorkflowManager      *workflow_manager.WorkflowManager
	RemoteBanSyncService *core.RemoteBanSyncService
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

		if config.Config.App.IsDevelopment && config.Config.App.WebUiProxy != "" {
			origin, _ := url.Parse(config.Config.App.WebUiProxy)

			director := func(req *http.Request) {
				req.Header.Add("X-Forwarded-Host", req.Host)
				req.Header.Add("X-Origin-Host", origin.Host)
				req.URL.Scheme = origin.Scheme
				req.URL.Host = origin.Host
			}

			proxy := &httputil.ReverseProxy{Director: director}
			proxy.ServeHTTP(w, r)
		} else {
			webEngine, err := web.New(serverDependencies.DB)
			if err != nil {
				log.Println("failed to create web engine", err)
			}
			webEngine.ServeHTTP(w, r)
		}
	}))

	// Setup route group for the API
	apiGroup := router.Group("/api")
	{
		apiGroup.GET("", server.apiHandler)
		apiGroup.GET("/", server.apiHandler)
		apiGroup.GET("/health", server.healthHandler)

		apiGroup.Use(server.OptionalAuthSession)

		apiGroup.GET("/images/avatar", server.GetAvatar)

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
			usersGroup.PUT("/:userId", server.UserUpdate)
			usersGroup.DELETE("/:userId", server.UserDelete)
		}

		// Ban List Management Routes
		banListsGroup := apiGroup.Group("/ban-lists")
		{
			banListsGroup.Use(server.AuthSession)
			banListsGroup.Use(server.AuthIsSuperAdmin())

			banListsGroup.GET("", server.BanListsList)
			banListsGroup.POST("", server.BanListsCreate)
			banListsGroup.GET("/:banListId", server.BanListsGet)
			banListsGroup.PUT("/:banListId", server.BanListsUpdate)
			banListsGroup.DELETE("/:banListId", server.BanListsDelete)
		}

		// Remote Ban Source Management Routes
		remoteBanSourcesGroup := apiGroup.Group("/remote-ban-sources")
		{
			remoteBanSourcesGroup.Use(server.AuthSession)
			remoteBanSourcesGroup.Use(server.AuthIsSuperAdmin())

			remoteBanSourcesGroup.GET("", server.RemoteBanSourcesList)
			remoteBanSourcesGroup.POST("", server.RemoteBanSourcesCreate)
			remoteBanSourcesGroup.PUT("/:sourceId", server.RemoteBanSourcesUpdate)
			remoteBanSourcesGroup.DELETE("/:sourceId", server.RemoteBanSourcesDelete)
		}

		// Ignored Steam ID Management Routes
		ignoredSteamIDsGroup := apiGroup.Group("/ignored-steam-ids")
		{
			ignoredSteamIDsGroup.Use(server.AuthSession)
			ignoredSteamIDsGroup.Use(server.AuthIsSuperAdmin())

			ignoredSteamIDsGroup.GET("", server.IgnoredSteamIDsList)
			ignoredSteamIDsGroup.POST("", server.IgnoredSteamIDsCreate)
			ignoredSteamIDsGroup.PUT("/:id", server.IgnoredSteamIDsUpdate)
			ignoredSteamIDsGroup.DELETE("/:id", server.IgnoredSteamIDsDelete)
			ignoredSteamIDsGroup.GET("/check/:steam_id", server.IgnoredSteamIDsCheck)
		}

		adminGroup := apiGroup.Group("/admin")
		{
			adminGroup.Use(server.AuthSession)
			adminGroup.Use(server.AuthIsSuperAdmin())

			adminGroup.POST("/cleanup-expired-admins", server.ServerAdminsCleanupExpired)
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
				serverGroup.PUT("", server.ServerUpdate)
				serverGroup.DELETE("", server.AuthIsSuperAdmin(), server.ServerDelete)

				serverGroup.GET("/metrics", server.ServerMetrics)
				serverGroup.GET("/metrics/history", server.ServerMetricsHistory)
				serverGroup.GET("/status", server.ServerStatus)
				serverGroup.GET("/audit-logs", server.AuthHasServerPermission("manageserver"), server.ServerAuditLogs)

				serverGroup.GET("/rcon/commands", server.RconCommandList)
				serverGroup.GET("/rcon/commands/autocomplete", server.RconCommandAutocomplete)
				serverGroup.POST("/rcon/execute", server.ServerRconExecute)
				serverGroup.GET("/rcon/server-population", server.ServerRconServerPopulation)
				serverGroup.GET("/rcon/available-layers", server.ServerRconAvailableLayers)
				serverGroup.GET("/rcon/events", server.AuthHasServerPermission("manageserver"), server.ServerRconEvents)
				serverGroup.POST("/rcon/force-restart", server.AuthHasServerPermission("manageserver"), server.ServerRconForceRestart)

				// Log watcher management
				serverGroup.POST("/logwatcher/restart", server.AuthHasServerPermission("manageserver"), server.ServerLogwatcherRestart)

				// Live feeds for chat, connections, and teamkills
				serverGroup.GET("/feeds", server.AuthSession, server.ServerFeeds)
				serverGroup.GET("/feeds/history", server.AuthSession, server.ServerFeedsHistory)

				serverGroup.GET("/roles", server.ServerRolesList)
				serverGroup.POST("/roles", server.AuthIsSuperAdmin(), server.ServerRolesAdd)
				serverGroup.DELETE("/roles/:roleId", server.AuthIsSuperAdmin(), server.ServerRolesRemove)

				serverGroup.GET("/admins", server.ServerAdminsList)
				serverGroup.POST("/admins", server.AuthIsSuperAdmin(), server.ServerAdminsAdd)
				serverGroup.PUT("/admins/:adminId", server.AuthIsSuperAdmin(), server.ServerAdminsUpdate)
				serverGroup.DELETE("/admins/:adminId", server.AuthIsSuperAdmin(), server.ServerAdminsRemove)

				serverGroup.GET("/bans", server.ServerBansList)
				serverGroup.POST("/bans", server.AuthHasAnyServerPermission("ban"), server.ServerBansAdd)
				serverGroup.PUT("/bans/:banId", server.AuthHasAnyServerPermission("ban"), server.ServerBansUpdate)
				serverGroup.DELETE("/bans/:banId", server.AuthHasAnyServerPermission("ban"), server.ServerBansRemove)

				// Ban list subscription management
				serverGroup.GET("/ban-list-subscriptions", server.ServerBanListSubscriptions)
				serverGroup.POST("/ban-list-subscriptions", server.AuthHasServerPermission("manageserver"), server.ServerBanListSubscriptionCreate)
				serverGroup.DELETE("/ban-list-subscriptions/:banListId", server.AuthHasServerPermission("manageserver"), server.ServerBanListSubscriptionDelete)

				// Player action endpoints
				serverGroup.POST("/rcon/kick-player", server.AuthHasAnyServerPermission("kick"), server.ServerRconKickPlayer)
				serverGroup.POST("/rcon/warn-player", server.AuthHasAnyServerPermission("kick"), server.ServerRconWarnPlayer)
				serverGroup.POST("/rcon/move-player", server.AuthHasAnyServerPermission("forceteamchange"), server.ServerRconMovePlayer)

				// Server info endpoints
				serverGroup.GET("/rcon/server-info", server.ServerRconServerInfo)

				// Plugin management routes for specific servers
				pluginGroup := serverGroup.Group("/plugins")
				{
					pluginGroup.GET("", server.ServerPluginList)
					pluginGroup.GET("/logs", server.AuthHasServerPermission("manageserver"), server.ServerPluginLogsAll)
					pluginGroup.POST("", server.AuthHasServerPermission("manageserver"), server.ServerPluginCreate)
					pluginGroup.GET("/:pluginId", server.ServerPluginGet)
					pluginGroup.PUT("/:pluginId", server.AuthHasServerPermission("manageserver"), server.ServerPluginUpdate)
					pluginGroup.POST("/:pluginId/enable", server.AuthHasServerPermission("manageserver"), server.ServerPluginEnable)
					pluginGroup.POST("/:pluginId/disable", server.AuthHasServerPermission("manageserver"), server.ServerPluginDisable)
					pluginGroup.DELETE("/:pluginId", server.AuthHasServerPermission("manageserver"), server.ServerPluginDelete)
					pluginGroup.GET("/:pluginId/logs", server.AuthHasServerPermission("manageserver"), server.ServerPluginLogs)
					pluginGroup.GET("/:pluginId/metrics", server.ServerPluginMetrics)
					pluginGroup.GET("/:pluginId/data", server.AuthHasServerPermission("manageserver"), server.ServerPluginDataGet)
					pluginGroup.POST("/:pluginId/data", server.AuthHasServerPermission("manageserver"), server.ServerPluginDataSet)
					pluginGroup.DELETE("/:pluginId/data", server.AuthHasServerPermission("manageserver"), server.ServerPluginDataClear)
					pluginGroup.DELETE("/:pluginId/data/:key", server.AuthHasServerPermission("manageserver"), server.ServerPluginDataDelete)
				}

				// Server Rules
				rulesGroup := serverGroup.Group("/rules")
				{
					rulesGroup.Use(server.AuthHasServerPermission("manageserver"))
					rulesGroup.GET("", server.listServerRules)
					rulesGroup.POST("", server.createServerRule)
					rulesGroup.PUT("/:ruleId", server.updateServerRule)
					rulesGroup.DELETE("/:ruleId", server.AuthIsSuperAdmin(), server.deleteServerRule)
					rulesGroup.PUT("/bulk", server.bulkUpdateServerRules) // Bulk update endpoint
				}

				// Server Workflows
				workflowsGroup := serverGroup.Group("/workflows")
				{
					workflowsGroup.Use(server.AuthHasServerPermission("manageserver"))
					workflowsGroup.GET("", server.ServerWorkflowsList)
					workflowsGroup.POST("", server.ServerWorkflowCreate)

					workflowGroup := workflowsGroup.Group("/:workflowId")
					{
						workflowGroup.GET("", server.ServerWorkflowGet)
						workflowGroup.PUT("", server.ServerWorkflowUpdate)
						workflowGroup.DELETE("", server.ServerWorkflowDelete)
						workflowGroup.POST("/execute", server.ServerWorkflowExecute)
						workflowGroup.GET("/executions", server.ServerWorkflowExecutions)

						// Workflow execution details and logs
						executionGroup := workflowGroup.Group("/executions/:executionId")
						{
							executionGroup.GET("", server.ServerWorkflowExecutionGet)
							executionGroup.GET("/logs", server.ServerWorkflowExecutionLogs)
							executionGroup.GET("/messages", server.ServerWorkflowExecutionMessages)
						}

						// Workflow variables
						variablesGroup := workflowGroup.Group("/variables")
						{
							variablesGroup.GET("", server.ServerWorkflowVariablesList)
							variablesGroup.POST("", server.ServerWorkflowVariableCreate)
							variablesGroup.PUT("/:variableId", server.ServerWorkflowVariableUpdate)
							variablesGroup.DELETE("/:variableId", server.ServerWorkflowVariableDelete)
						}
					}
				}
			}
		}

		// Global plugin and connector management routes
		pluginsGroup := apiGroup.Group("/plugins")
		{
			pluginsGroup.Use(server.AuthSession)
			pluginsGroup.Use(server.AuthIsSuperAdmin())

			pluginsGroup.GET("/available", server.PluginListAvailable)
		}

		connectorsGroup := apiGroup.Group("/connectors")
		{
			connectorsGroup.Use(server.AuthSession)
			connectorsGroup.Use(server.AuthIsSuperAdmin())

			connectorsGroup.GET("/available", server.ConnectorListAvailable)
			connectorsGroup.GET("", server.ConnectorList)
			connectorsGroup.POST("", server.ConnectorCreate)
			connectorsGroup.PUT("/:connectorId", server.ConnectorUpdate)
			connectorsGroup.DELETE("/:connectorId", server.ConnectorDelete)
		}

		// Public Routes for the server
		apiGroup.GET("/servers/:serverId/admins/cfg", server.ServerAdminsCfg)
		apiGroup.GET("/servers/:serverId/bans/cfg", server.ServerBansCfgEnhanced)
		apiGroup.GET("/ban-lists/:banListId/cfg", server.BanListCfg)
	}

	return router
}

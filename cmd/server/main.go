// Yoinked some code from woodpecker/woodpecker/cmd/server/server.go

package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/samber/oops"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/internal/clickhouse"
	"go.codycody31.dev/squad-aegis/internal/core"
	"go.codycody31.dev/squad-aegis/internal/db"
	"go.codycody31.dev/squad-aegis/internal/event_manager"
	"go.codycody31.dev/squad-aegis/internal/logwatcher_manager"
	"go.codycody31.dev/squad-aegis/internal/models"
	"go.codycody31.dev/squad-aegis/internal/plugin_manager"
	"go.codycody31.dev/squad-aegis/internal/plugin_registry"
	"go.codycody31.dev/squad-aegis/internal/rcon_manager"
	"go.codycody31.dev/squad-aegis/internal/server"
	"go.codycody31.dev/squad-aegis/internal/shared/config"
	"go.codycody31.dev/squad-aegis/internal/shared/logger"
	"go.codycody31.dev/squad-aegis/internal/shared/utils"
	"golang.org/x/sync/errgroup"
)

const (
	shutdownTimeout = time.Second * 5
)

var (
	stopServerFunc     context.CancelCauseFunc = func(error) {}
	shutdownCancelFunc context.CancelFunc      = func() {}
	shutdownCtx                                = context.Background()
)

func main() {
	ctx := utils.WithContextSigtermCallback(context.Background(), func() {
		log.Info().Msg("termination signal is received, shutting down server")
	})

	if err := run(ctx); err != nil {
		log.Error().Msgf("error running Squad Aegis: %v", err)
	}
}

func run(ctx context.Context) error {
	ctx, ctxCancel := context.WithCancelCause(ctx)
	stopServerFunc = func(err error) {
		if err != nil {
			log.Error().Err(err).Msg("shutdown of whole server")
		}
		stopServerFunc = func(error) {}
		shutdownCtx, shutdownCancelFunc = context.WithTimeout(shutdownCtx, shutdownTimeout)
		ctxCancel(err)
	}
	defer stopServerFunc(nil)
	defer shutdownCancelFunc()

	err := logger.SetupGlobalLogger(ctx, config.Config.Log.Level, config.Config.Debug.Pretty, config.Config.Debug.NoColor, config.Config.Log.File, true)
	if err != nil {
		return fmt.Errorf("failed to set up logger: %v", err)
	}

	// set gin mode based on log level
	if zerolog.GlobalLevel() > zerolog.DebugLevel {
		gin.SetMode(gin.ReleaseMode)
	}

	log.Info().Msg("Starting Squad Aegis...")

	// Initialize database
	var database *sql.DB
	postgresDSN := db.PostgresDSN(config.Config.Db.Host, config.Config.Db.Port, config.Config.Db.User, config.Config.Db.Pass, config.Config.Db.Name)
	log.Trace().Msgf("Connecting to database: %s", postgresDSN)
	database, err = sql.Open("postgres", postgresDSN)
	if err != nil {
		return fmt.Errorf("failed to open database: %v", err)
	}
	defer database.Close()

	// Ping database
	if err = database.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %v", err)
	}

	// Use transaction to ensure atomicity of setup
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %v", err)
	}

	// Database migration
	log.Info().Msg("Migrating database...")
	err = db.Migrate(database, config.Config.Db.Migrate.Verbose)
	if err != nil {
		return fmt.Errorf("failed to migrate database: %v", err)
	}

	// Setup if not already setup
	if err := setup(tx); err != nil {
		if err := tx.Rollback(); err != nil {
			return fmt.Errorf("failed to rollback transaction: %v", err)
		}
		return fmt.Errorf("failed to setup: %v", err)
	}

	// Commit transaction
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %v", err)
	}

	// Create event manager for centralized event handling
	eventManager := event_manager.NewEventManager(ctx, 10000)
	defer eventManager.Shutdown()

	// Initialize ClickHouse
	var clickhouseClient *clickhouse.Client
	var eventIngester *clickhouse.EventIngester
	clickhouseConfig := clickhouse.Config{
		Host:     config.Config.ClickHouse.Host,
		Port:     config.Config.ClickHouse.Port,
		Database: config.Config.ClickHouse.Database,
		Username: config.Config.ClickHouse.Username,
		Password: config.Config.ClickHouse.Password,
		Debug:    config.Config.ClickHouse.Debug,
	}

	clickhouseClient, err = clickhouse.NewClient(clickhouseConfig)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to ClickHouse")
	}
	defer clickhouseClient.Close()

	// Ping clickhouse
	err = clickhouseClient.Ping(ctx)
	if err != nil {
		return oops.Wrapf(err, "failed to ping clickhouse")
	}

	// Migrate clickhouse
	log.Info().Msg("Migrating clickhouse...")
	err = clickhouse.Migrate(clickhouseClient.GetConnection(), config.Config.ClickHouse.Debug)
	if err != nil {
		return oops.Wrapf(err, "failed to migrate clickhouse")
	}

	// Create and start event ingester
	eventIngester = clickhouse.NewEventIngester(ctx, clickhouseClient, eventManager)
	eventIngester.Start()
	defer eventIngester.Stop()

	// Create RCON manager
	rconManager := rcon_manager.NewRconManager(ctx, eventManager)
	defer rconManager.Shutdown()

	// Create logwatcher manager
	logwatcherManager := logwatcher_manager.NewLogwatcherManager(ctx, eventManager)
	defer logwatcherManager.Shutdown()

	// Create plugin manager
	pluginManager := plugin_manager.NewPluginManager(ctx, database, eventManager, rconManager, clickhouseClient)
	defer pluginManager.Stop()

	// Register all available plugins and connectors
	if err := plugin_registry.RegisterAllConnectors(pluginManager); err != nil {
		return fmt.Errorf("failed to register connectors: %w", err)
	}

	if err := plugin_registry.RegisterAllPlugins(pluginManager); err != nil {
		return fmt.Errorf("failed to register plugins: %w", err)
	}

	// Start plugin manager
	if err := pluginManager.Start(); err != nil {
		return fmt.Errorf("failed to start plugin manager: %w", err)
	}

	// Start connection managers
	go rconManager.StartConnectionManager()
	go logwatcherManager.StartConnectionManager()

	// Start admin cleanup task (runs every hour)
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()

		// Run cleanup immediately on startup
		go func() {
			if deleted, err := core.CleanupExpiredAdmins(ctx, database); err != nil {
				log.Error().Err(err).Msg("failed to cleanup expired admins")
			} else if deleted > 0 {
				log.Info().Int64("deleted", deleted).Msg("cleaned up expired admin roles")
			}
		}()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if deleted, err := core.CleanupExpiredAdmins(ctx, database); err != nil {
					log.Error().Err(err).Msg("failed to cleanup expired admins")
				} else if deleted > 0 {
					log.Info().Int64("deleted", deleted).Msg("cleaned up expired admin roles")
				}
			}
		}
	}()

	// Connect to all servers
	rconManager.ConnectToAllServers(ctx, database)
	logwatcherManager.ConnectToAllServers(ctx, database)

	// Initialize services
	waitingGroup := errgroup.Group{}

	// Start HTTP server
	waitingGroup.Go(func() error {
		log.Info().Msg("Starting http service...")

		httpPort := config.Config.App.Port
		if httpPort == "" {
			httpPort = "3131"
		}

		deps := &server.Dependencies{
			DB:                   database,
			RconManager:          rconManager,
			EventManager:         eventManager,
			LogwatcherManager:    logwatcherManager,
			PluginManager:        pluginManager,
			RemoteBanSyncService: core.NewRemoteBanSyncService(database, database),
		}

		// Start remote ban sync service
		go deps.RemoteBanSyncService.StartPeriodicSync(ctx) // Initialize router
		router := server.NewRouter(deps)

		// Create server with timeout
		srv := &http.Server{
			Addr:              ":" + httpPort,
			Handler:           router,
			ReadHeaderTimeout: 5 * time.Second,
		}

		// Run server in a goroutine to allow for graceful shutdown
		serverErrChan := make(chan error, 1)
		go func() {
			log.Info().Msgf("http server listening on http://localhost:%s", httpPort)
			if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				serverErrChan <- err
			}
			close(serverErrChan)
		}()

		select {
		case <-ctx.Done():
			log.Info().Msg("shutting down http server...")
			shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
			defer cancel()

			if err := srv.Shutdown(shutdownCtx); err != nil {
				log.Error().Err(err).Msg("error shutting down http server")
				return err
			}

			log.Info().Msg("http server shutdown completed")
			return nil
		case err := <-serverErrChan:
			if err != nil {
				log.Error().Err(err).Msg("http server error")
				return err
			}
		}

		log.Info().Msg("http service stopped")
		return nil
	})

	return waitingGroup.Wait()
}

func setup(database db.Executor) error {
	adminId, err := uuid.NewUUID()
	if err != nil {
		return fmt.Errorf("failed to generate admin user id: %v", err)
	}

	// Create admin user if not exists
	admin := &models.User{
		Id:         adminId,
		Username:   config.Config.Initial.Admin.Username,
		Password:   config.Config.Initial.Admin.Password,
		SuperAdmin: true,
	}

	if _, err := core.GetUserByUsername(context.Background(), database, admin.Username, nil); err != nil {
		if err != core.ErrorUserNotFound {
			return fmt.Errorf("failed to get admin user: %v", err)
		}
	} else {
		log.Info().Msg("admin user already exists, skipping registration")
		return nil
	}

	if _, err := core.RegisterUser(context.Background(), database, admin); err != nil {
		return fmt.Errorf("failed to register admin user: %v", err)
	}

	log.Info().Msg("admin user registered")

	return nil
}

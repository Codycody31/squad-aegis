// Yoinked some code from woodpecker/woodpecker/cmd/server/server.go

package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"time"

	_ "github.com/lib/pq"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/connectors/discord"
	"go.codycody31.dev/squad-aegis/core"
	"go.codycody31.dev/squad-aegis/db"
	"go.codycody31.dev/squad-aegis/extensions/core/discord_admin_request"
	"go.codycody31.dev/squad-aegis/extensions/core/discord_chat"
	"go.codycody31.dev/squad-aegis/internal/connector_manager"
	"go.codycody31.dev/squad-aegis/internal/extension_manager"
	"go.codycody31.dev/squad-aegis/internal/models"
	"go.codycody31.dev/squad-aegis/internal/rcon_manager"
	"go.codycody31.dev/squad-aegis/internal/server"
	"go.codycody31.dev/squad-aegis/shared/config"
	"go.codycody31.dev/squad-aegis/shared/logger"
	"go.codycody31.dev/squad-aegis/shared/utils"
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

	// Initialize RCON manager
	rconManager := rcon_manager.NewRconManager(ctx)

	// Initialize connector manager
	connectorManager := connector_manager.NewConnectorManager(ctx)

	// Register connector factories
	connectorManager.RegisterFactory(discord.Factory)

	// Initialize connectors from database
	if err := connectorManager.InitializeConnectors(ctx, database); err != nil {
		log.Error().Err(err).Msg("Failed to initialize connectors")
	}

	// Initialize extension manager
	extensionManager := extension_manager.NewExtensionManager(ctx, connectorManager, rconManager)

	// Register extension factories
	extensionManager.RegisterFactory("discord_admin_request", discord_admin_request.Factory)
	extensionManager.RegisterFactory("discord_chat", discord_chat.Factory)

	// Initialize extensions from database
	if err := extensionManager.InitializeExtensions(ctx, database); err != nil {
		log.Error().Err(err).Msg("Failed to initialize extensions")
	}

	// Initialize services
	waitingGroup := errgroup.Group{}

	waitingGroup.Go(func() error {
		log.Info().Msg("Starting RCON connection manager service...")
		go rconManager.StartConnectionManager()

		// Connect to all servers with RCON enabled
		log.Info().Msg("Connecting to all servers with RCON enabled...")
		rconManager.ConnectToAllServers(ctx, database)

		log.Info().Msg("RCON connection manager service started")

		// Block until context is done
		<-ctx.Done()

		// Stop RCON connection manager
		rconManager.Shutdown()

		log.Info().Msg("RCON connection manager service stopped")

		return nil
	})

	// Start extension event listener
	waitingGroup.Go(func() error {
		log.Info().Msg("Starting extension event listener...")

		// Start extension event listener
		extensionManager.StartEventListener()

		log.Info().Msg("Extension event listener started")

		// Block until context is done
		<-ctx.Done()

		// Stop extension manager
		extensionManager.Shutdown()

		log.Info().Msg("Extension manager stopped")
		return nil
	})

	// Start HTTP server
	waitingGroup.Go(func() error {
		log.Info().Msg("Starting http service...")

		httpPort := config.Config.App.Port
		if httpPort == "" {
			httpPort = "3131"
		}

		deps := &server.Dependencies{
			DB:               database,
			RconManager:      rconManager,
			ConnectorManager: connectorManager,
			ExtensionManager: extensionManager,
		}

		// Initialize router
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
	// Create admin user if not exists
	admin := &models.User{
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

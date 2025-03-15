package chat_processor

import (
	"context"
	"database/sql"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/internal/rcon"
	"go.codycody31.dev/squad-aegis/internal/rcon_manager"
)

// Command represents a chat command
type Command struct {
	Name        string
	Description string
	Usage       string
	Handler     func(ctx context.Context, serverID uuid.UUID, message rcon.Message, args []string) (string, error)
	AdminOnly   bool
}

// ChatProcessor processes chat messages and handles commands
type ChatProcessor struct {
	rconManager *rcon_manager.RconManager
	db          *sql.DB
	commands    map[string]Command
	mu          sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
}

// NewChatProcessor creates a new chat processor
func NewChatProcessor(ctx context.Context, rconManager *rcon_manager.RconManager, db *sql.DB) *ChatProcessor {
	ctx, cancel := context.WithCancel(ctx)
	return &ChatProcessor{
		rconManager: rconManager,
		db:          db,
		commands:    make(map[string]Command),
		ctx:         ctx,
		cancel:      cancel,
	}
}

// RegisterCommand registers a command
func (p *ChatProcessor) RegisterCommand(command Command) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.commands[strings.ToLower(command.Name)] = command
}

// Start starts the chat processor
func (p *ChatProcessor) Start() {
	// Register default commands
	p.registerDefaultCommands()

	// Start processing chat messages
	p.rconManager.ProcessChatMessages(p.ctx, p.handleChatMessage)

	log.Info().Msg("Chat processor started")
}

// Stop stops the chat processor
func (p *ChatProcessor) Stop() {
	p.cancel()
	log.Info().Msg("Chat processor stopped")
}

// handleChatMessage handles a chat message
func (p *ChatProcessor) handleChatMessage(serverID uuid.UUID, message rcon.Message) {
	// Log the message
	log.Debug().
		Str("serverID", serverID.String()).
		Str("chatType", message.ChatType).
		Str("playerName", message.PlayerName).
		Str("message", message.Message).
		Msg("Chat message received")

	// Check if the message is a command
	if !strings.HasPrefix(message.Message, "!") {
		return
	}

	// Parse the command
	parts := strings.Fields(message.Message[1:])
	if len(parts) == 0 {
		return
	}

	commandName := strings.ToLower(parts[0])
	args := parts[1:]

	// Get the command
	p.mu.RLock()
	command, exists := p.commands[commandName]
	p.mu.RUnlock()

	if !exists {
		return
	}

	// Check if the command is admin-only
	if command.AdminOnly {
		isAdmin, err := p.isPlayerAdmin(p.ctx, serverID, message.SteamID)
		if err != nil {
			log.Error().
				Err(err).
				Str("serverID", serverID.String()).
				Str("steamID", message.SteamID).
				Msg("Failed to check if player is admin")
			return
		}

		if !isAdmin {
			// Send a message to the player
			p.sendDirectMessage(serverID, "You don't have permission to use this command.", message.SteamID)
			return
		}
	}

	// Execute the command
	response, err := command.Handler(p.ctx, serverID, message, args)
	if err != nil {
		log.Error().
			Err(err).
			Str("serverID", serverID.String()).
			Str("command", commandName).
			Msg("Failed to execute command")

		// Send error message to the player
		p.sendDirectMessage(serverID, "Something went wrong, please try again later.", message.SteamID)
		return
	}

	// Send the response to the player if there is one
	if response != "" {
		p.sendDirectMessage(serverID, response, message.SteamID)
	}
}

// isPlayerAdmin checks if a player is an admin
func (p *ChatProcessor) isPlayerAdmin(ctx context.Context, serverID uuid.UUID, steamID string) (bool, error) {
	// Query the database to check if the player is an admin
	var count int
	err := p.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM server_admins sa
		JOIN users u ON sa.user_id = u.id
		WHERE sa.server_id = $1 AND u.steam_id = $2
	`, serverID, steamID).Scan(&count)

	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// registerDefaultCommands registers the default commands
func (p *ChatProcessor) registerDefaultCommands() {
	p.RegisterCommand(Command{
		Name:        "admin",
		Description: "Calls an admin",
		Usage:       "!admin [message]",
		Handler:     p.handleAdminCommand,
		AdminOnly:   false,
	})
}

// handleAdminCommand handles the admin command
func (p *ChatProcessor) handleAdminCommand(ctx context.Context, serverID uuid.UUID, message rcon.Message, args []string) (string, error) {
	// Get all admins for the server
	rows, err := p.db.QueryContext(ctx, `
		SELECT u.username, u.steam_id
		FROM server_admins sa
		JOIN users u ON sa.user_id = u.id
		WHERE sa.server_id = $1
	`, serverID)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	// Build the admin message
	adminMessage := message.PlayerName + " is requesting admin assistance"
	if len(args) > 0 {
		adminMessage += ": " + strings.Join(args, " ")
	}

	// TODO: Discord integration: ie: send message to discord channel

	// Send a message to all admins
	for rows.Next() {
		var username string
		var steamID string
		if err := rows.Scan(&username, &steamID); err != nil {
			log.Error().
				Err(err).
				Str("serverID", serverID.String()).
				Msg("Failed to scan admin row")
			continue
		}

		// Send a direct message to the admin
		p.sendDirectMessage(serverID, adminMessage, steamID)
	}

	if err := rows.Err(); err != nil {
		log.Error().
			Err(err).
			Str("serverID", serverID.String()).
			Msg("Error iterating admin rows")
	}

	return "Your request has been sent to the admins.", nil
}

// sendDirectMessage sends a direct message to a player
func (p *ChatProcessor) sendDirectMessage(serverID uuid.UUID, message string, steamID string) {
	// Format the command to send a direct message to the player
	command := "AdminWarn " + steamID + " " + message

	// Execute the command
	_, err := p.rconManager.ExecuteCommand(serverID, command)
	if err != nil {
		log.Error().
			Err(err).
			Str("serverID", serverID.String()).
			Str("steamID", steamID).
			Str("message", message).
			Msg("Failed to send direct message")
	}
}

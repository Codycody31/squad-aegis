package server

import (
	"path"
	"path/filepath"
	"strings"
	"time"

	"go.codycody31.dev/squad-aegis/internal/logwatcher_manager"
	"go.codycody31.dev/squad-aegis/internal/models"
)

const (
	squadGameLogsRelPath  = "Saved/Logs/SquadGame.log"
	squadGameBansRelPath  = "ServerConfig/Bans.cfg"
	squadGameMotdRelPath  = "ServerConfig/MOTD.cfg"
)

func buildServerPath(basePath string, useSlash bool, relPath string) string {
	if useSlash {
		// SFTP/FTP always use forward slashes. Normalize any Windows-style
		// backslashes the user may have entered in their SquadGamePath.
		return path.Join(strings.ReplaceAll(basePath, `\`, "/"), relPath)
	}
	return filepath.Join(basePath, filepath.FromSlash(relPath))
}

func buildLogFilePath(basePath string, logSourceType *string) string {
	useSlash := logSourceType != nil && (*logSourceType == "sftp" || *logSourceType == "ftp")
	return buildServerPath(basePath, useSlash, squadGameLogsRelPath)
}

func buildBansCfgPath(basePath string, logSourceType *string) string {
	useSlash := logSourceType != nil && (*logSourceType == "sftp" || *logSourceType == "ftp")
	return buildServerPath(basePath, useSlash, squadGameBansRelPath)
}

func buildMotdPath(basePath string, protocol string) string {
	useSlash := protocol == "sftp" || protocol == "ftp"
	return buildServerPath(basePath, useSlash, squadGameMotdRelPath)
}

func buildLogSourceConfig(server *models.Server) logwatcher_manager.LogSourceConfig {
	config := logwatcher_manager.LogSourceConfig{
		Type:          logwatcher_manager.LogSourceType(*server.LogSourceType),
		FilePath:      buildLogFilePath(*server.SquadGamePath, server.LogSourceType),
		ReadFromStart: false,
	}

	if server.LogHost != nil {
		config.Host = *server.LogHost
	}
	if server.LogPort != nil {
		config.Port = *server.LogPort
	}
	if server.LogUsername != nil {
		config.Username = *server.LogUsername
	}
	if server.LogPassword != nil {
		config.Password = *server.LogPassword
	}
	if server.LogPollFrequency != nil {
		config.PollFrequency = time.Duration(*server.LogPollFrequency) * time.Second
	}
	if server.LogReadFromStart != nil {
		config.ReadFromStart = *server.LogReadFromStart
	}

	return config
}

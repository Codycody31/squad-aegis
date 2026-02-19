package server

import (
	"time"

	"go.codycody31.dev/squad-aegis/internal/logwatcher_manager"
	"go.codycody31.dev/squad-aegis/internal/models"
	"go.codycody31.dev/squad-aegis/internal/shared/utils"
)

func buildLogFilePath(basePath string, logSourceType *string) string {
	return utils.BuildSquadServerPath(basePath, utils.IsRemoteProtocolPtr(logSourceType), utils.SquadGameLogsRelPath)
}

func buildBansCfgPath(basePath string, logSourceType *string) string {
	return utils.BuildSquadServerPath(basePath, utils.IsRemoteProtocolPtr(logSourceType), utils.SquadGameBansRelPath)
}

func buildMotdPath(basePath string, protocol string) string {
	return utils.BuildSquadServerPath(basePath, utils.IsRemoteProtocol(protocol), utils.SquadGameMotdRelPath)
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

package server

import (
	"path"
	"path/filepath"
)

const (
	squadGameLogsRelPath  = "Saved/Logs/SquadGame.log"
	squadGameBansRelPath  = "ServerConfig/Bans.cfg"
	squadGameMotdRelPath  = "ServerConfig/MOTD.cfg"
)

func buildServerPath(basePath string, useSlash bool, relPath string) string {
	if useSlash {
		return path.Join(basePath, relPath)
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

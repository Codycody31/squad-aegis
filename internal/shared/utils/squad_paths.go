package utils

import (
	"path"
	"path/filepath"
	"strings"
)

// Well-known relative paths within a Squad dedicated server installation.
const (
	SquadGameLogsRelPath = "Saved/Logs/SquadGame.log"
	SquadGameBansRelPath = "ServerConfig/Bans.cfg"
	SquadGameMotdRelPath = "ServerConfig/MOTD.cfg"
)

// BuildSquadServerPath joins basePath and relPath using the correct separator
// for the transport. SFTP/FTP always use forward slashes; local paths use the
// OS-native separator.
func BuildSquadServerPath(basePath string, useSlash bool, relPath string) string {
	if useSlash {
		// SFTP/FTP always use forward slashes. Normalize any Windows-style
		// backslashes the user may have entered in their SquadGamePath.
		return path.Join(strings.ReplaceAll(basePath, `\`, "/"), relPath)
	}
	return filepath.Join(basePath, filepath.FromSlash(relPath))
}

// IsRemoteProtocol returns true if the protocol requires forward-slash paths
// (SFTP or FTP).
func IsRemoteProtocol(protocol string) bool {
	return protocol == "sftp" || protocol == "ftp"
}

// IsRemoteProtocolPtr is a nil-safe variant of IsRemoteProtocol.
func IsRemoteProtocolPtr(protocol *string) bool {
	return protocol != nil && IsRemoteProtocol(*protocol)
}

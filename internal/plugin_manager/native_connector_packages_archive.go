package plugin_manager

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func readConnectorBundle(archive io.ReaderAt, size int64) (ConnectorPackageManifest, PluginPackageTarget, []byte, []byte, []byte, []byte, string, error) {
	reader, err := zip.NewReader(archive, size)
	if err != nil {
		return ConnectorPackageManifest{}, PluginPackageTarget{}, nil, nil, nil, nil, "", fmt.Errorf("invalid connector archive: %w", err)
	}

	if len(reader.File) > pluginArchiveMaxEntries {
		return ConnectorPackageManifest{}, PluginPackageTarget{}, nil, nil, nil, nil, "", fmt.Errorf("connector archive has %d entries, exceeding limit of %d", len(reader.File), pluginArchiveMaxEntries)
	}

	var manifest ConnectorPackageManifest
	var manifestBytes []byte
	var signatureBytes []byte
	var publicKeyBytes []byte
	libraries := make(map[string]*zip.File)
	var manifestFile *zip.File
	var signatureFile *zip.File
	var publicKeyFile *zip.File

	for _, file := range reader.File {
		name := filepath.Clean(strings.TrimPrefix(file.Name, "/"))
		if name == "." || strings.HasPrefix(name, "..") || filepath.IsAbs(name) {
			return ConnectorPackageManifest{}, PluginPackageTarget{}, nil, nil, nil, nil, "", fmt.Errorf("connector archive contains an unsafe path: %s", file.Name)
		}
		if file.FileInfo().Mode()&os.ModeSymlink != 0 {
			return ConnectorPackageManifest{}, PluginPackageTarget{}, nil, nil, nil, nil, "", fmt.Errorf("connector archive contains an unsupported symlink: %s", file.Name)
		}

		switch filepath.Base(name) {
		case pluginManifestFile:
			manifestFile = file
		case pluginSignatureFile:
			signatureFile = file
		case pluginPublicKeyFile:
			publicKeyFile = file
		default:
			lower := strings.ToLower(name)
			if strings.HasSuffix(lower, ".so") {
				libraries[name] = file
			}
		}
	}

	if manifestFile == nil {
		return ConnectorPackageManifest{}, PluginPackageTarget{}, nil, nil, nil, nil, "", fmt.Errorf("connector archive is missing %s", pluginManifestFile)
	}

	budget := newPluginArchiveReadBudget()

	manifestBytes, err = budget.read(manifestFile)
	if err != nil {
		return ConnectorPackageManifest{}, PluginPackageTarget{}, nil, nil, nil, nil, "", err
	}
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return ConnectorPackageManifest{}, PluginPackageTarget{}, nil, nil, nil, nil, "", fmt.Errorf("invalid connector manifest: %w", err)
	}

	selectedTarget, err := selectedConnectorManifestTarget(manifest)
	if err != nil {
		return ConnectorPackageManifest{}, PluginPackageTarget{}, nil, nil, nil, nil, "", err
	}

	if signatureFile != nil {
		signatureBytes, err = budget.read(signatureFile)
		if err != nil {
			return ConnectorPackageManifest{}, PluginPackageTarget{}, nil, nil, nil, nil, "", err
		}
	}
	if publicKeyFile != nil {
		publicKeyBytes, err = budget.read(publicKeyFile)
		if err != nil {
			return ConnectorPackageManifest{}, PluginPackageTarget{}, nil, nil, nil, nil, "", err
		}
	}

	hasSignature := len(bytes.TrimSpace(signatureBytes)) > 0
	hasPublicKey := len(bytes.TrimSpace(publicKeyBytes)) > 0
	if hasSignature != hasPublicKey {
		return ConnectorPackageManifest{}, PluginPackageTarget{}, nil, nil, nil, nil, "", fmt.Errorf("connector archive must include %s and %s together", pluginSignatureFile, pluginPublicKeyFile)
	}

	libraryName, libraryFile, err := selectManifestLibrary(manifest.asPluginManifest(), selectedTarget, libraries)
	if err != nil {
		return ConnectorPackageManifest{}, PluginPackageTarget{}, nil, nil, nil, nil, "", err
	}
	libraryBytes, err := budget.read(libraryFile)
	if err != nil {
		return ConnectorPackageManifest{}, PluginPackageTarget{}, nil, nil, nil, nil, "", err
	}

	return manifest, selectedTarget, manifestBytes, signatureBytes, publicKeyBytes, libraryBytes, libraryName, nil
}

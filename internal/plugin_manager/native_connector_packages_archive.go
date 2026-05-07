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

// connectorBundleParts mirrors pluginBundleParts for connector bundles.
type connectorBundleParts struct {
	Manifest       ConnectorPackageManifest
	SelectedTarget PluginPackageTarget
	ManifestBytes  []byte
	SignedPayload  []byte
	SignatureBytes []byte
	PublicKeyBytes []byte
	LibraryBytes   []byte
	LibraryName    string
}

func readConnectorBundle(archive io.ReaderAt, size int64) (connectorBundleParts, error) {
	reader, err := zip.NewReader(archive, size)
	if err != nil {
		return connectorBundleParts{}, fmt.Errorf("invalid connector archive: %w", err)
	}

	if len(reader.File) > pluginArchiveMaxEntries {
		return connectorBundleParts{}, fmt.Errorf("connector archive has %d entries, exceeding limit of %d", len(reader.File), pluginArchiveMaxEntries)
	}

	libraries := make(map[string]*zip.File)
	var manifestFile *zip.File
	var signedFile *zip.File
	var signatureFile *zip.File
	var publicKeyFile *zip.File

	for _, file := range reader.File {
		name := filepath.Clean(strings.TrimPrefix(file.Name, "/"))
		if name == "." || strings.HasPrefix(name, "..") || filepath.IsAbs(name) {
			return connectorBundleParts{}, fmt.Errorf("connector archive contains an unsafe path: %s", file.Name)
		}
		if file.FileInfo().Mode()&os.ModeSymlink != 0 {
			return connectorBundleParts{}, fmt.Errorf("connector archive contains an unsupported symlink: %s", file.Name)
		}

		// Manifest, signed payload, signature and public key files are only
		// accepted at the archive root, mirroring the plugin reader. See
		// native_packages_archive.go for the rationale.
		isRoot := !strings.ContainsRune(name, filepath.Separator)
		switch {
		case isRoot && name == pluginManifestFile:
			if manifestFile != nil {
				return connectorBundleParts{}, fmt.Errorf("connector archive contains multiple %s entries", pluginManifestFile)
			}
			manifestFile = file
		case isRoot && name == pluginSignedManifestFile:
			if signedFile != nil {
				return connectorBundleParts{}, fmt.Errorf("connector archive contains multiple %s entries", pluginSignedManifestFile)
			}
			signedFile = file
		case isRoot && name == pluginSignatureFile:
			if signatureFile != nil {
				return connectorBundleParts{}, fmt.Errorf("connector archive contains multiple %s entries", pluginSignatureFile)
			}
			signatureFile = file
		case isRoot && name == pluginPublicKeyFile:
			if publicKeyFile != nil {
				return connectorBundleParts{}, fmt.Errorf("connector archive contains multiple %s entries", pluginPublicKeyFile)
			}
			publicKeyFile = file
		default:
			base := filepath.Base(name)
			if base == pluginManifestFile || base == pluginSignedManifestFile || base == pluginSignatureFile || base == pluginPublicKeyFile {
				return connectorBundleParts{}, fmt.Errorf("connector archive must place %s at the archive root, found %s", base, file.Name)
			}
			// Accept every non-metadata file as a potential runtime binary;
			// manifest.library_path disambiguates the entrypoint.
			libraries[name] = file
		}
	}

	if manifestFile == nil {
		return connectorBundleParts{}, fmt.Errorf("connector archive is missing %s", pluginManifestFile)
	}

	budget := newPluginArchiveReadBudget()

	manifestBytes, err := budget.read(manifestFile)
	if err != nil {
		return connectorBundleParts{}, err
	}
	var manifest ConnectorPackageManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return connectorBundleParts{}, fmt.Errorf("invalid connector manifest: %w", err)
	}

	selectedTarget, err := selectedConnectorManifestTarget(manifest)
	if err != nil {
		return connectorBundleParts{}, err
	}

	var signedPayload []byte
	if signedFile != nil {
		signedPayload, err = budget.read(signedFile)
		if err != nil {
			return connectorBundleParts{}, err
		}
	}
	var signatureBytes []byte
	if signatureFile != nil {
		signatureBytes, err = budget.read(signatureFile)
		if err != nil {
			return connectorBundleParts{}, err
		}
	}
	var publicKeyBytes []byte
	if publicKeyFile != nil {
		publicKeyBytes, err = budget.read(publicKeyFile)
		if err != nil {
			return connectorBundleParts{}, err
		}
	}

	hasSignedPayload := len(bytes.TrimSpace(signedPayload)) > 0
	hasSignature := len(bytes.TrimSpace(signatureBytes)) > 0
	hasPublicKey := len(bytes.TrimSpace(publicKeyBytes)) > 0
	signingArtefactCount := 0
	for _, present := range []bool{hasSignedPayload, hasSignature, hasPublicKey} {
		if present {
			signingArtefactCount++
		}
	}
	if signingArtefactCount != 0 && signingArtefactCount != 3 {
		return connectorBundleParts{}, fmt.Errorf("connector archive must include %s, %s, and %s together", pluginSignedManifestFile, pluginSignatureFile, pluginPublicKeyFile)
	}

	libraryName, libraryFile, err := selectManifestLibrary(manifest.asPluginManifest(), selectedTarget, libraries)
	if err != nil {
		return connectorBundleParts{}, err
	}
	libraryBytes, err := budget.read(libraryFile)
	if err != nil {
		return connectorBundleParts{}, err
	}

	return connectorBundleParts{
		Manifest:       manifest,
		SelectedTarget: selectedTarget,
		ManifestBytes:  manifestBytes,
		SignedPayload:  signedPayload,
		SignatureBytes: signatureBytes,
		PublicKeyBytes: publicKeyBytes,
		LibraryBytes:   libraryBytes,
		LibraryName:    libraryName,
	}, nil
}

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

type pluginArchiveReadBudget struct {
	entryLimit int64
	totalLimit int64
	remaining  int64
}

func newPluginArchiveReadBudget() pluginArchiveReadBudget {
	totalLimit := pluginMaxArchiveUncompressedSize()
	return pluginArchiveReadBudget{
		entryLimit: pluginMaxUploadSize(),
		totalLimit: totalLimit,
		remaining:  totalLimit,
	}
}

func (b *pluginArchiveReadBudget) read(file *zip.File) ([]byte, error) {
	if file == nil {
		return nil, nil
	}

	if file.UncompressedSize64 > uint64(b.entryLimit) {
		return nil, fmt.Errorf("plugin archive entry %s exceeds uncompressed size limit of %d bytes", file.Name, b.entryLimit)
	}
	if file.UncompressedSize64 > uint64(b.remaining) {
		return nil, fmt.Errorf("plugin archive exceeds total uncompressed size limit of %d bytes", b.totalLimit)
	}

	rc, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open archive entry %s: %w", file.Name, err)
	}

	readLimit := b.entryLimit
	totalLimited := false
	if b.remaining < readLimit {
		readLimit = b.remaining
		totalLimited = true
	}

	limited := &io.LimitedReader{R: rc, N: readLimit + 1}
	data, readErr := io.ReadAll(limited)
	closeErr := rc.Close()
	if readErr != nil {
		return nil, fmt.Errorf("failed to read archive entry %s: %w", file.Name, readErr)
	}
	if int64(len(data)) > readLimit {
		if totalLimited {
			return nil, fmt.Errorf("plugin archive exceeds total uncompressed size limit of %d bytes", b.totalLimit)
		}
		return nil, fmt.Errorf("plugin archive entry %s exceeds uncompressed size limit of %d bytes", file.Name, b.entryLimit)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("failed to close archive entry %s: %w", file.Name, closeErr)
	}

	b.remaining -= int64(len(data))

	return data, nil
}

func readPluginBundle(archive io.ReaderAt, size int64) (PluginPackageManifest, PluginPackageTarget, []byte, []byte, []byte, []byte, string, error) {
	reader, err := zip.NewReader(archive, size)
	if err != nil {
		return PluginPackageManifest{}, PluginPackageTarget{}, nil, nil, nil, nil, "", fmt.Errorf("invalid plugin archive: %w", err)
	}

	if len(reader.File) > pluginArchiveMaxEntries {
		return PluginPackageManifest{}, PluginPackageTarget{}, nil, nil, nil, nil, "", fmt.Errorf("plugin archive has %d entries, exceeding limit of %d", len(reader.File), pluginArchiveMaxEntries)
	}

	var manifest PluginPackageManifest
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
			return PluginPackageManifest{}, PluginPackageTarget{}, nil, nil, nil, nil, "", fmt.Errorf("plugin archive contains an unsafe path: %s", file.Name)
		}
		if file.FileInfo().Mode()&os.ModeSymlink != 0 {
			return PluginPackageManifest{}, PluginPackageTarget{}, nil, nil, nil, nil, "", fmt.Errorf("plugin archive contains an unsupported symlink: %s", file.Name)
		}

		// Manifest, signature and public key files are accepted only at the
		// archive root. This prevents bundles from shipping multiple
		// candidates (e.g. `manifest.json` AND `subdir/manifest.json`) where
		// the last entry seen would silently win and bypass operator review.
		isRoot := !strings.ContainsRune(name, filepath.Separator)
		switch {
		case isRoot && name == pluginManifestFile:
			if manifestFile != nil {
				return PluginPackageManifest{}, PluginPackageTarget{}, nil, nil, nil, nil, "", fmt.Errorf("plugin archive contains multiple %s entries", pluginManifestFile)
			}
			manifestFile = file
		case isRoot && name == pluginSignatureFile:
			if signatureFile != nil {
				return PluginPackageManifest{}, PluginPackageTarget{}, nil, nil, nil, nil, "", fmt.Errorf("plugin archive contains multiple %s entries", pluginSignatureFile)
			}
			signatureFile = file
		case isRoot && name == pluginPublicKeyFile:
			if publicKeyFile != nil {
				return PluginPackageManifest{}, PluginPackageTarget{}, nil, nil, nil, nil, "", fmt.Errorf("plugin archive contains multiple %s entries", pluginPublicKeyFile)
			}
			publicKeyFile = file
		default:
			// Reject non-root manifest/sig/pubkey files outright; nothing
			// legitimate ships them in subdirectories.
			base := filepath.Base(name)
			if base == pluginManifestFile || base == pluginSignatureFile || base == pluginPublicKeyFile {
				return PluginPackageManifest{}, PluginPackageTarget{}, nil, nil, nil, nil, "", fmt.Errorf("plugin archive must place %s at the archive root, found %s", base, file.Name)
			}
			// Everything else is a candidate runtime binary. The manifest's
			// library_path field disambiguates exactly which one is the
			// plugin entrypoint, so we intentionally accept all non-metadata
			// files here rather than filtering by suffix.
			libraries[name] = file
		}
	}

	if manifestFile == nil {
		return PluginPackageManifest{}, PluginPackageTarget{}, nil, nil, nil, nil, "", fmt.Errorf("plugin archive is missing %s", pluginManifestFile)
	}

	budget := newPluginArchiveReadBudget()

	manifestBytes, err = budget.read(manifestFile)
	if err != nil {
		return PluginPackageManifest{}, PluginPackageTarget{}, nil, nil, nil, nil, "", err
	}
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return PluginPackageManifest{}, PluginPackageTarget{}, nil, nil, nil, nil, "", fmt.Errorf("invalid plugin manifest: %w", err)
	}

	selectedTarget, err := selectedManifestTarget(manifest)
	if err != nil {
		return PluginPackageManifest{}, PluginPackageTarget{}, nil, nil, nil, nil, "", err
	}

	if signatureFile != nil {
		signatureBytes, err = budget.read(signatureFile)
		if err != nil {
			return PluginPackageManifest{}, PluginPackageTarget{}, nil, nil, nil, nil, "", err
		}
	}
	if publicKeyFile != nil {
		publicKeyBytes, err = budget.read(publicKeyFile)
		if err != nil {
			return PluginPackageManifest{}, PluginPackageTarget{}, nil, nil, nil, nil, "", err
		}
	}

	hasSignature := len(bytes.TrimSpace(signatureBytes)) > 0
	hasPublicKey := len(bytes.TrimSpace(publicKeyBytes)) > 0
	if hasSignature != hasPublicKey {
		return PluginPackageManifest{}, PluginPackageTarget{}, nil, nil, nil, nil, "", fmt.Errorf("plugin archive must include %s and %s together", pluginSignatureFile, pluginPublicKeyFile)
	}

	libraryName, libraryFile, err := selectManifestLibrary(manifest, selectedTarget, libraries)
	if err != nil {
		return PluginPackageManifest{}, PluginPackageTarget{}, nil, nil, nil, nil, "", err
	}
	libraryBytes, err := budget.read(libraryFile)
	if err != nil {
		return PluginPackageManifest{}, PluginPackageTarget{}, nil, nil, nil, nil, "", err
	}

	return manifest, selectedTarget, manifestBytes, signatureBytes, publicKeyBytes, libraryBytes, libraryName, nil
}

func selectManifestLibrary(manifest PluginPackageManifest, target PluginPackageTarget, libraries map[string]*zip.File) (string, *zip.File, error) {
	libraryPath := strings.TrimSpace(target.LibraryPath)
	if libraryPath != "" {
		declaredPath := filepath.Clean(strings.TrimPrefix(libraryPath, "/"))
		libraryFile := libraries[declaredPath]
		if libraryFile == nil {
			return "", nil, fmt.Errorf("plugin archive is missing declared library %s", libraryPath)
		}
		return declaredPath, libraryFile, nil
	}

	if len(libraries) == 1 {
		for name, file := range libraries {
			return name, file, nil
		}
	}

	if len(libraries) == 0 {
		return "", nil, fmt.Errorf("plugin archive is missing a plugin binary")
	}

	return "", nil, fmt.Errorf("plugin manifest target %s/%s with min_host_api_version %d is missing library_path", target.TargetOS, target.TargetArch, target.MinHostAPIVersion)
}

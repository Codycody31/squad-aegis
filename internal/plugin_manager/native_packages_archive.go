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

// pluginBundleParts is the set of byte slices the install pipeline needs
// after walking the archive: the parsed manifest and selected target plus
// the raw bytes for hashing/signing/verification.
type pluginBundleParts struct {
	Manifest          PluginPackageManifest
	SelectedTarget    PluginPackageTarget
	ManifestBytes     []byte
	SignedPayload     []byte
	SignatureBytes    []byte
	PublicKeyBytes    []byte
	LibraryBytes      []byte
	LibraryName       string
}

func readPluginBundle(archive io.ReaderAt, size int64) (pluginBundleParts, error) {
	reader, err := zip.NewReader(archive, size)
	if err != nil {
		return pluginBundleParts{}, fmt.Errorf("invalid plugin archive: %w", err)
	}

	if len(reader.File) > pluginArchiveMaxEntries {
		return pluginBundleParts{}, fmt.Errorf("plugin archive has %d entries, exceeding limit of %d", len(reader.File), pluginArchiveMaxEntries)
	}

	libraries := make(map[string]*zip.File)
	var manifestFile *zip.File
	var signedFile *zip.File
	var signatureFile *zip.File
	var publicKeyFile *zip.File

	for _, file := range reader.File {
		name := filepath.Clean(strings.TrimPrefix(file.Name, "/"))
		if name == "." || strings.HasPrefix(name, "..") || filepath.IsAbs(name) {
			return pluginBundleParts{}, fmt.Errorf("plugin archive contains an unsafe path: %s", file.Name)
		}
		if file.FileInfo().Mode()&os.ModeSymlink != 0 {
			return pluginBundleParts{}, fmt.Errorf("plugin archive contains an unsupported symlink: %s", file.Name)
		}

		// Manifest, signed payload, signature and public key files are
		// accepted only at the archive root. This prevents bundles from
		// shipping multiple candidates where the last entry seen would
		// silently win and bypass operator review.
		isRoot := !strings.ContainsRune(name, filepath.Separator)
		switch {
		case isRoot && name == pluginManifestFile:
			if manifestFile != nil {
				return pluginBundleParts{}, fmt.Errorf("plugin archive contains multiple %s entries", pluginManifestFile)
			}
			manifestFile = file
		case isRoot && name == pluginSignedManifestFile:
			if signedFile != nil {
				return pluginBundleParts{}, fmt.Errorf("plugin archive contains multiple %s entries", pluginSignedManifestFile)
			}
			signedFile = file
		case isRoot && name == pluginSignatureFile:
			if signatureFile != nil {
				return pluginBundleParts{}, fmt.Errorf("plugin archive contains multiple %s entries", pluginSignatureFile)
			}
			signatureFile = file
		case isRoot && name == pluginPublicKeyFile:
			if publicKeyFile != nil {
				return pluginBundleParts{}, fmt.Errorf("plugin archive contains multiple %s entries", pluginPublicKeyFile)
			}
			publicKeyFile = file
		default:
			// Reject non-root manifest/signed/sig/pubkey files outright;
			// nothing legitimate ships them in subdirectories.
			base := filepath.Base(name)
			if base == pluginManifestFile || base == pluginSignedManifestFile || base == pluginSignatureFile || base == pluginPublicKeyFile {
				return pluginBundleParts{}, fmt.Errorf("plugin archive must place %s at the archive root, found %s", base, file.Name)
			}
			// Everything else is a candidate runtime binary. The manifest's
			// library_path field disambiguates exactly which one is the
			// plugin entrypoint, so we intentionally accept all non-metadata
			// files here rather than filtering by suffix.
			libraries[name] = file
		}
	}

	if manifestFile == nil {
		return pluginBundleParts{}, fmt.Errorf("plugin archive is missing %s", pluginManifestFile)
	}

	budget := newPluginArchiveReadBudget()

	manifestBytes, err := budget.read(manifestFile)
	if err != nil {
		return pluginBundleParts{}, err
	}
	var manifest PluginPackageManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return pluginBundleParts{}, fmt.Errorf("invalid plugin manifest: %w", err)
	}

	selectedTarget, err := selectedManifestTarget(manifest)
	if err != nil {
		return pluginBundleParts{}, err
	}

	var signedPayload []byte
	if signedFile != nil {
		signedPayload, err = budget.read(signedFile)
		if err != nil {
			return pluginBundleParts{}, err
		}
	}

	var signatureBytes []byte
	if signatureFile != nil {
		signatureBytes, err = budget.read(signatureFile)
		if err != nil {
			return pluginBundleParts{}, err
		}
	}
	var publicKeyBytes []byte
	if publicKeyFile != nil {
		publicKeyBytes, err = budget.read(publicKeyFile)
		if err != nil {
			return pluginBundleParts{}, err
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
		return pluginBundleParts{}, fmt.Errorf("plugin archive must include %s, %s, and %s together", pluginSignedManifestFile, pluginSignatureFile, pluginPublicKeyFile)
	}

	libraryName, libraryFile, err := selectManifestLibrary(manifest, selectedTarget, libraries)
	if err != nil {
		return pluginBundleParts{}, err
	}
	libraryBytes, err := budget.read(libraryFile)
	if err != nil {
		return pluginBundleParts{}, err
	}

	return pluginBundleParts{
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

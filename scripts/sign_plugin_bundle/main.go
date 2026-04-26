package main

import (
	"archive/zip"
	"crypto/ed25519"
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"go.codycody31.dev/squad-aegis/internal/plugin_signing"
)

func main() {
	bundleDir := flag.String("bundle-dir", "", "directory containing manifest.json and the plugin .so")
	privateKeyPath := flag.String("private-key", "", "path to the base64-encoded ed25519 private key file")
	outputZip := flag.String("output", "", "path to the signed zip bundle to create")
	keyID := flag.String("key-id", "", "operator-chosen identifier for the signing key (e.g. ops-key-2026-q1); recorded in the signed payload and matched against the host CRL")
	validFor := flag.Duration("valid-for", 365*24*time.Hour, "how long the signature should be valid (Go duration format, e.g. 8760h)")
	flag.Parse()

	if err := run(*bundleDir, *privateKeyPath, *outputZip, *keyID, *validFor); err != nil {
		fmt.Fprintf(os.Stderr, "sign plugin bundle: %v\n", err)
		os.Exit(1)
	}
}

func run(bundleDir, privateKeyPath, outputZip, keyID string, validFor time.Duration) error {
	if strings.TrimSpace(bundleDir) == "" {
		return fmt.Errorf("bundle-dir is required")
	}
	if strings.TrimSpace(privateKeyPath) == "" {
		return fmt.Errorf("private-key is required")
	}
	if strings.TrimSpace(outputZip) == "" {
		return fmt.Errorf("output is required")
	}
	if strings.TrimSpace(keyID) == "" {
		return fmt.Errorf("key-id is required")
	}
	if validFor <= 0 {
		return fmt.Errorf("valid-for must be positive")
	}

	manifestPath := filepath.Join(bundleDir, "manifest.json")
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to read manifest.json: %w", err)
	}

	privateKeyBytes, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return fmt.Errorf("failed to read private key: %w", err)
	}

	privateKeyRaw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(privateKeyBytes)))
	if err != nil {
		return fmt.Errorf("failed to decode private key: %w", err)
	}

	privateKey := ed25519.PrivateKey(privateKeyRaw)
	if len(privateKey) != ed25519.PrivateKeySize {
		return fmt.Errorf("private key has %d bytes, want %d", len(privateKey), ed25519.PrivateKeySize)
	}

	signedAt := time.Now().UTC()
	expiresAt := signedAt.Add(validFor)

	signedPayload, err := plugin_signing.BuildSignedPayload(manifestBytes, keyID, signedAt, expiresAt)
	if err != nil {
		return fmt.Errorf("failed to build signed payload: %w", err)
	}

	signatureFile, publicKeyFile, err := plugin_signing.SignSignedPayload(signedPayload, privateKey)
	if err != nil {
		return err
	}

	signedPayloadPath := filepath.Join(bundleDir, plugin_signing.SignedManifestPayloadFile)
	if err := os.WriteFile(signedPayloadPath, signedPayload, 0o644); err != nil {
		return fmt.Errorf("failed to write %s: %w", plugin_signing.SignedManifestPayloadFile, err)
	}
	signaturePath := filepath.Join(bundleDir, plugin_signing.ManifestSignatureFile)
	if err := os.WriteFile(signaturePath, signatureFile, 0o644); err != nil {
		return fmt.Errorf("failed to write manifest.sig: %w", err)
	}
	publicKeyPath := filepath.Join(bundleDir, plugin_signing.ManifestPublicKeyFile)
	if err := os.WriteFile(publicKeyPath, publicKeyFile, 0o644); err != nil {
		return fmt.Errorf("failed to write manifest.pub: %w", err)
	}

	files, err := collectBundleFiles(bundleDir)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(outputZip), 0o755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}
	if err := writeZip(outputZip, bundleDir, files); err != nil {
		return err
	}

	fmt.Printf("Signed bundle created: %s\n", outputZip)
	fmt.Printf("Signed payload file: %s\n", signedPayloadPath)
	fmt.Printf("Signature file: %s\n", signaturePath)
	fmt.Printf("Public key file: %s\n", publicKeyPath)
	fmt.Printf("Key ID: %s\n", keyID)
	fmt.Printf("Signed at: %s\n", signedAt.Format(time.RFC3339))
	fmt.Printf("Expires at: %s\n", expiresAt.Format(time.RFC3339))

	return nil
}

// collectBundleFiles walks bundleDir and selects only the files that belong
// in a signed bundle: manifest.json, manifest.signed.json, manifest.sig,
// manifest.pub at the root, and any *.so under bin/. It refuses any file
// whose name suggests it might be a private key or credential to defend
// against an operator accidentally dropping a private key inside the bundle
// dir before zipping.
func collectBundleFiles(bundleDir string) ([]string, error) {
	allowedRoots := map[string]struct{}{
		"manifest.json":                         {},
		plugin_signing.SignedManifestPayloadFile: {},
		plugin_signing.ManifestSignatureFile:    {},
		plugin_signing.ManifestPublicKeyFile:    {},
	}

	var files []string

	err := filepath.WalkDir(bundleDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(d.Name(), ".zip") {
			return nil
		}

		relativePath, err := filepath.Rel(bundleDir, path)
		if err != nil {
			return fmt.Errorf("failed to compute relative path for %s: %w", path, err)
		}
		rel := filepath.ToSlash(relativePath)
		base := filepath.Base(rel)
		lower := strings.ToLower(base)

		// Refuse anything that looks like a private key, credential, or
		// other secret. The bundle should never contain these, but operators
		// have been known to leave them next to manifest.json.
		if isLikelySecretFile(lower) {
			return fmt.Errorf("refusing to bundle %s: filename looks like a secret/private key. Move it outside %s before signing", rel, bundleDir)
		}

		// Allow manifest/signed/sig/pubkey only at archive root.
		if _, ok := allowedRoots[base]; ok {
			if strings.ContainsRune(rel, '/') {
				return fmt.Errorf("refusing to bundle %s: %s must live at the bundle root", rel, base)
			}
			files = append(files, rel)
			return nil
		}

		// Any file under bin/ is accepted. The layout is deterministic
		// because the manifest's library_path field pinpoints the
		// subprocess executable, and the SHA-256 is bound by the signed
		// manifest. Subprocess-isolated plugins are plain Go binaries, so
		// there is no fixed extension we can rely on.
		if strings.HasPrefix(rel, "bin/") {
			files = append(files, rel)
			return nil
		}
		_ = lower

		// Anything else is rejected. Operators must explicitly stage what
		// they want signed.
		return fmt.Errorf("refusing to bundle %s: only manifest.json, %s, %s, %s, and files under bin/ are accepted", rel, plugin_signing.SignedManifestPayloadFile, plugin_signing.ManifestSignatureFile, plugin_signing.ManifestPublicKeyFile)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to collect bundle files: %w", err)
	}

	sort.Strings(files)
	return files, nil
}

// isLikelySecretFile returns true when the lowercased basename matches
// common patterns for private keys, credentials, or other secrets that must
// never end up inside a signed bundle.
func isLikelySecretFile(lowerBase string) bool {
	switch lowerBase {
	case "id_rsa", "id_dsa", "id_ecdsa", "id_ed25519",
		".env", "credentials", "credentials.json",
		"secret", "secret.txt", "secrets.json":
		return true
	}
	suffixes := []string{".pem", ".key", ".p12", ".pfx", ".jks", ".kdbx"}
	for _, suffix := range suffixes {
		if strings.HasSuffix(lowerBase, suffix) {
			return true
		}
	}
	prefixes := []string{"private", "priv-", "priv_"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(lowerBase, prefix) {
			return true
		}
	}
	return false
}

func writeZip(outputZip, bundleDir string, files []string) error {
	outputFile, err := os.Create(outputZip)
	if err != nil {
		return fmt.Errorf("failed to create output zip: %w", err)
	}
	defer outputFile.Close()

	zipWriter := zip.NewWriter(outputFile)
	defer zipWriter.Close()

	for _, relativePath := range files {
		absolutePath := filepath.Join(bundleDir, filepath.FromSlash(relativePath))
		sourceFile, err := os.Open(absolutePath)
		if err != nil {
			return fmt.Errorf("failed to open %s: %w", absolutePath, err)
		}

		fileInfo, err := sourceFile.Stat()
		if err != nil {
			sourceFile.Close()
			return fmt.Errorf("failed to stat %s: %w", absolutePath, err)
		}

		header, err := zip.FileInfoHeader(fileInfo)
		if err != nil {
			sourceFile.Close()
			return fmt.Errorf("failed to create zip header for %s: %w", absolutePath, err)
		}
		header.Name = relativePath
		header.Method = zip.Deflate

		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			sourceFile.Close()
			return fmt.Errorf("failed to create zip entry for %s: %w", absolutePath, err)
		}

		if _, err := io.Copy(writer, sourceFile); err != nil {
			sourceFile.Close()
			return fmt.Errorf("failed to add %s to zip: %w", absolutePath, err)
		}
		if err := sourceFile.Close(); err != nil {
			return fmt.Errorf("failed to close %s: %w", absolutePath, err)
		}
	}

	if err := zipWriter.Close(); err != nil {
		return fmt.Errorf("failed to finalize output zip: %w", err)
	}

	return nil
}

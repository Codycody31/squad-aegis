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

	"go.codycody31.dev/squad-aegis/internal/plugin_signing"
)

func main() {
	bundleDir := flag.String("bundle-dir", "", "directory containing manifest.json and the plugin .so")
	privateKeyPath := flag.String("private-key", "", "path to the base64-encoded ed25519 private key file")
	outputZip := flag.String("output", "", "path to the signed zip bundle to create")
	flag.Parse()

	if err := run(*bundleDir, *privateKeyPath, *outputZip); err != nil {
		fmt.Fprintf(os.Stderr, "sign plugin bundle: %v\n", err)
		os.Exit(1)
	}
}

func run(bundleDir, privateKeyPath, outputZip string) error {
	if strings.TrimSpace(bundleDir) == "" {
		return fmt.Errorf("bundle-dir is required")
	}
	if strings.TrimSpace(privateKeyPath) == "" {
		return fmt.Errorf("private-key is required")
	}
	if strings.TrimSpace(outputZip) == "" {
		return fmt.Errorf("output is required")
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

	signatureFile, publicKeyFile, err := plugin_signing.SignManifest(manifestBytes, privateKey)
	if err != nil {
		return err
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
	fmt.Printf("Signature file: %s\n", signaturePath)
	fmt.Printf("Public key file: %s\n", publicKeyPath)

	return nil
}

func collectBundleFiles(bundleDir string) ([]string, error) {
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
		files = append(files, filepath.ToSlash(relativePath))
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to collect bundle files: %w", err)
	}

	sort.Strings(files)
	return files, nil
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

package plugin_manager

import (
	"crypto/subtle"
	"fmt"
	"strings"

	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/internal/plugin_signing"
	"go.codycody31.dev/squad-aegis/internal/shared/config"
)

// verifyManifestSignature rejects any manifest.pub not in the operator-
// configured trust store before delegating to the cryptographic verifier.
// Without this anchor the signed-sideload gate is bypassable, since an
// attacker could ship their own keypair inside the bundle.
func verifyManifestSignature(manifestBytes, signatureBytes, publicKeyBytes []byte) (bool, error) {
	if len(signatureBytes) == 0 || len(publicKeyBytes) == 0 {
		return false, nil
	}
	if !isTrustedSigningKey(publicKeyBytes) {
		return false, fmt.Errorf("manifest signed by untrusted public key")
	}
	return plugin_signing.VerifyManifest(manifestBytes, signatureBytes, publicKeyBytes)
}

func isTrustedSigningKey(publicKeyBytes []byte) bool {
	if config.Config == nil {
		return false
	}
	raw := strings.TrimSpace(config.Config.Plugins.TrustedSigningKeys)
	if raw == "" {
		return false
	}

	candidate, err := plugin_signing.DecodePublicKeyFile(publicKeyBytes)
	if err != nil {
		return false
	}

	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		trusted, err := plugin_signing.DecodePublicKeyString(entry)
		if err != nil {
			log.Warn().Err(err).Str("key", entry).Msg("Ignoring invalid trusted plugin signing key")
			continue
		}
		if subtleConstantTimeEqual(candidate, trusted) {
			return true
		}
	}
	return false
}

func subtleConstantTimeEqual(a, b []byte) bool {
	return len(a) == len(b) && subtle.ConstantTimeCompare(a, b) == 1
}

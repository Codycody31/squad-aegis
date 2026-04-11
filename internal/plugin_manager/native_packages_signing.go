package plugin_manager

import (
	"crypto/subtle"
	"strings"

	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/internal/plugin_signing"
	"go.codycody31.dev/squad-aegis/internal/shared/config"
)

// verifyManifestSignature rejects any manifest.pub not in the operator-
// configured trust store before delegating to the cryptographic verifier.
// Without this anchor the signed-sideload gate is bypassable, since an
// attacker could ship their own keypair inside the bundle.
//
// Returns:
//   - (false, nil) when the bundle is unsigned, signed by an untrusted key,
//     or the cryptographic check fails. These cases are uniformly treated as
//     "not verified" so the AllowUnsafeSideload gate can apply consistently.
//   - (false, err) ONLY when canonicalization or input parsing produces an
//     unrecoverable error (malformed inputs). Callers should treat this as a
//     hard failure rather than a "not verified" outcome.
//   - (true, nil) when the signature checks out against a trusted key.
func verifyManifestSignature(manifestBytes, signatureBytes, publicKeyBytes []byte) (bool, error) {
	if len(signatureBytes) == 0 || len(publicKeyBytes) == 0 {
		return false, nil
	}
	if !isTrustedSigningKey(publicKeyBytes) {
		log.Warn().Msg("Plugin manifest is signed by a key that is not in the trusted signing key list; treating as unverified")
		return false, nil
	}
	return plugin_signing.VerifyManifest(manifestBytes, signatureBytes, publicKeyBytes)
}

// nativePluginsConfigured reports whether the native plugin runtime has any
// trust anchor configured. Returns true if either at least one trusted
// signing key is set, or AllowUnsafeSideload is explicitly enabled.
func nativePluginsConfigured() bool {
	if config.Config == nil {
		return false
	}
	if config.Config.Plugins.AllowUnsafeSideload {
		return true
	}
	return strings.TrimSpace(config.Config.Plugins.TrustedSigningKeys) != ""
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

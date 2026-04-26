package plugin_manager

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/internal/plugin_signing"
	"go.codycody31.dev/squad-aegis/internal/shared/config"
)

// builtinTrustedKeys are always trusted regardless of operator configuration.
// This is the official Squad Aegis signing public key.
var builtinTrustedKeys = []string{
	"ATziv2ugm/CzbnnYSLZGsq/eJfCxHI8mu/Ot19RGmD0=",
}

// signatureVerification carries the resolved verdict for a bundle's signed
// payload: whether the signature checks out against a trusted, non-revoked,
// non-expired key, and the wrapper metadata so callers can log/persist it.
type signatureVerification struct {
	Verified bool
	Payload  plugin_signing.SignedManifestPayload
}

// verifyManifestSignature checks the signed payload bundled alongside the
// manifest. The bundle layout is:
//
//   - manifest.json - human-inspectable manifest (unsigned alone)
//   - manifest.signed.json - canonical wrapper carrying manifest + key_id +
//     signed_at + expires_at; THIS is what the signature covers
//   - manifest.sig - ed25519 signature over manifest.signed.json
//   - manifest.pub - public key of the signer
//
// Returns:
//   - (verified=false, nil) when the bundle is unsigned, signed by an
//     untrusted key, expired, revoked, or the cryptographic check fails. The
//     AllowUnsafeSideload gate then applies uniformly.
//   - (_, err) ONLY when canonicalization or input parsing produces an
//     unrecoverable error. Callers should treat this as a hard failure.
func verifyManifestSignature(signedPayloadBytes, manifestBytes, signatureBytes, publicKeyBytes []byte) (signatureVerification, error) {
	if len(signatureBytes) == 0 || len(publicKeyBytes) == 0 || len(signedPayloadBytes) == 0 {
		return signatureVerification{}, nil
	}
	if !isTrustedSigningKey(publicKeyBytes) {
		log.Warn().Msg("Plugin manifest is signed by a key that is not in the trusted signing key list; treating as unverified")
		return signatureVerification{}, nil
	}

	payload, ok, err := plugin_signing.VerifySignedPayload(signedPayloadBytes, manifestBytes, signatureBytes, publicKeyBytes)
	if err != nil {
		return signatureVerification{}, err
	}
	if !ok {
		return signatureVerification{}, nil
	}

	skew := signatureClockSkew()
	now := time.Now().UTC()
	if now.After(payload.ExpiresAt.Add(skew)) {
		log.Warn().Str("key_id", payload.KeyID).Time("expires_at", payload.ExpiresAt).Msg("Plugin signature is expired; treating as unverified")
		return signatureVerification{Payload: payload}, nil
	}

	if isRevokedKeyID(payload.KeyID) {
		log.Warn().Str("key_id", payload.KeyID).Msg("Plugin signature key_id is revoked; treating as unverified")
		return signatureVerification{Payload: payload}, nil
	}

	return signatureVerification{Verified: true, Payload: payload}, nil
}

func isTrustedSigningKey(publicKeyBytes []byte) bool {
	candidate, err := plugin_signing.DecodePublicKeyFile(publicKeyBytes)
	if err != nil {
		return false
	}

	// Check builtin keys first.
	for _, entry := range builtinTrustedKeys {
		trusted, err := plugin_signing.DecodePublicKeyString(entry)
		if err != nil {
			continue
		}
		if subtleConstantTimeEqual(candidate, trusted) {
			return true
		}
	}

	// Check operator-configured keys.
	if config.Config == nil {
		return false
	}
	raw := strings.TrimSpace(config.Config.Plugins.TrustedSigningKeys)
	if raw == "" {
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

func signatureClockSkew() time.Duration {
	if config.Config == nil {
		return 0
	}
	seconds := config.Config.Plugins.SignatureClockSkewSeconds
	if seconds <= 0 {
		return 0
	}
	return time.Duration(seconds) * time.Second
}

// keyRevocationList is the parsed CRL kept in memory and refreshed in the
// background. The mutex protects the revoked map so isRevokedKeyID can be
// called from any goroutine without coordinating with the refresher.
type keyRevocationList struct {
	mu      sync.RWMutex
	revoked map[string]struct{}
}

var globalRevocationList = &keyRevocationList{revoked: make(map[string]struct{})}

type revokedKeyIDsFile struct {
	RevokedKeyIDs []string `json:"revoked_key_ids"`
}

// isRevokedKeyID reports whether keyID appears in the operator-managed CRL.
// Returns false when the CRL is unconfigured.
func isRevokedKeyID(keyID string) bool {
	keyID = strings.TrimSpace(keyID)
	if keyID == "" {
		return false
	}
	globalRevocationList.mu.RLock()
	defer globalRevocationList.mu.RUnlock()
	_, ok := globalRevocationList.revoked[keyID]
	return ok
}

// loadRevokedKeyIDs reads the CRL file once and replaces the in-memory set.
// An empty path clears the set. Errors are logged but not returned so a
// missing or transiently-unreadable CRL never wedges plugin loading; the
// previous snapshot stays in effect until the next successful refresh.
func loadRevokedKeyIDs() {
	path := ""
	if config.Config != nil {
		path = strings.TrimSpace(config.Config.Plugins.RevokedKeyIDsPath)
	}
	if path == "" {
		globalRevocationList.mu.Lock()
		globalRevocationList.revoked = make(map[string]struct{})
		globalRevocationList.mu.Unlock()
		return
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		log.Warn().Err(err).Str("path", path).Msg("Failed to read plugin signing CRL; previous snapshot kept")
		return
	}

	var parsed revokedKeyIDsFile
	if err := json.Unmarshal(raw, &parsed); err != nil {
		log.Warn().Err(err).Str("path", path).Msg("Failed to parse plugin signing CRL; previous snapshot kept")
		return
	}

	next := make(map[string]struct{}, len(parsed.RevokedKeyIDs))
	for _, id := range parsed.RevokedKeyIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		next[id] = struct{}{}
	}

	globalRevocationList.mu.Lock()
	globalRevocationList.revoked = next
	globalRevocationList.mu.Unlock()
	log.Info().Str("path", path).Int("count", len(next)).Msg("Loaded plugin signing CRL")
}

// startRevokedKeyIDsRefresher loads the CRL once and then re-reads it on the
// configured interval until ctx is cancelled. Safe to call once from
// PluginManager.Start.
func startRevokedKeyIDsRefresher(ctx interface {
	Done() <-chan struct{}
}) {
	loadRevokedKeyIDs()

	intervalSeconds := 300
	if config.Config != nil && config.Config.Plugins.RevokedKeyIDsRefreshSeconds > 0 {
		intervalSeconds = config.Config.Plugins.RevokedKeyIDsRefreshSeconds
	}
	if intervalSeconds < 5 {
		intervalSeconds = 5
	}
	interval := time.Duration(intervalSeconds) * time.Second

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				loadRevokedKeyIDs()
			}
		}
	}()
}

// resetRevokedKeyIDsForTest empties the in-memory CRL. Tests that exercise
// the CRL path use this to seed and tear down state without going through
// disk.
func resetRevokedKeyIDsForTest(revoked ...string) {
	next := make(map[string]struct{}, len(revoked))
	for _, id := range revoked {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		next[id] = struct{}{}
	}
	globalRevocationList.mu.Lock()
	globalRevocationList.revoked = next
	globalRevocationList.mu.Unlock()
}

// formatVerificationFailure builds a clear quarantine message tailored to
// why the signature did not verify.
func formatVerificationFailure(payload plugin_signing.SignedManifestPayload) string {
	if payload.KeyID == "" {
		return "plugin signature did not verify"
	}
	if isRevokedKeyID(payload.KeyID) {
		return fmt.Sprintf("plugin signature key %q is revoked", payload.KeyID)
	}
	if !payload.ExpiresAt.IsZero() && time.Now().UTC().After(payload.ExpiresAt.Add(signatureClockSkew())) {
		return fmt.Sprintf("plugin signature expired at %s (key %q)", payload.ExpiresAt.Format(time.RFC3339), payload.KeyID)
	}
	return fmt.Sprintf("plugin signature for key %q did not verify", payload.KeyID)
}

package plugin_manager

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
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

// verifyManifestSignature checks the signed payload bundled with the
// manifest. The bundle ships manifest.json (display), manifest.signed.json
// (canonical wrapper covering manifest + key_id + signed_at + expires_at,
// which the signature is over), manifest.sig, and manifest.pub.
//
// Returns (verified=false, nil) for unsigned, untrusted-key, expired,
// revoked, or crypto-failure cases so AllowUnsafeSideload can gate them
// uniformly. Returns an error only on canonicalization or parse failures.
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

	// Check both the operator-supplied key_id (a self-declared label inside
	// the signed payload) and the fingerprint of the actual signing key. The
	// fingerprint binds revocation to the cryptographic identity, so a
	// compromised but otherwise-trusted public key can be revoked even if
	// the attacker chooses a fresh key_id on subsequent bundles.
	if isRevokedKeyID(payload.KeyID) {
		log.Warn().Str("key_id", payload.KeyID).Msg("Plugin signature key_id is revoked; treating as unverified")
		return signatureVerification{Payload: payload}, nil
	}
	if isRevokedPublicKey(publicKeyBytes) {
		log.Warn().Str("key_id", payload.KeyID).Str("fingerprint", publicKeyFingerprint(publicKeyBytes)).Msg("Plugin signing public key is revoked; treating as unverified")
		return signatureVerification{Payload: payload}, nil
	}

	return signatureVerification{Verified: true, Payload: payload}, nil
}

// publicKeyFingerprint returns a SHA256 hex of the decoded public key bytes,
// or "" if the key cannot be decoded. Used both for CRL lookup and for
// human-readable logging.
func publicKeyFingerprint(publicKeyBytes []byte) string {
	pk, err := plugin_signing.DecodePublicKeyFile(publicKeyBytes)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(pk)
	return hex.EncodeToString(sum[:])
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

// keyRevocationList is the in-memory CRL refreshed in the background. The
// mutex lets lookup helpers run without coordinating with the refresher.
// revokedKeyIDs match the operator-supplied key_id label (bypassable by
// re-signing with a different label); revokedFingerprints match the SHA256
// of the public key bytes and authoritatively retire a compromised key.
type keyRevocationList struct {
	mu                  sync.RWMutex
	revokedKeyIDs       map[string]struct{}
	revokedFingerprints map[string]struct{}
}

var globalRevocationList = &keyRevocationList{
	revokedKeyIDs:       make(map[string]struct{}),
	revokedFingerprints: make(map[string]struct{}),
}

type revokedKeyIDsFile struct {
	RevokedKeyIDs       []string `json:"revoked_key_ids"`
	RevokedFingerprints []string `json:"revoked_public_key_fingerprints"`
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
	_, ok := globalRevocationList.revokedKeyIDs[keyID]
	return ok
}

// isRevokedPublicKey reports whether the SHA256 fingerprint of publicKeyBytes
// appears in the operator-managed CRL. Returns false when the CRL is
// unconfigured or the key bytes do not decode.
func isRevokedPublicKey(publicKeyBytes []byte) bool {
	fingerprint := publicKeyFingerprint(publicKeyBytes)
	if fingerprint == "" {
		return false
	}
	globalRevocationList.mu.RLock()
	defer globalRevocationList.mu.RUnlock()
	_, ok := globalRevocationList.revokedFingerprints[fingerprint]
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
		globalRevocationList.revokedKeyIDs = make(map[string]struct{})
		globalRevocationList.revokedFingerprints = make(map[string]struct{})
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

	nextIDs := make(map[string]struct{}, len(parsed.RevokedKeyIDs))
	for _, id := range parsed.RevokedKeyIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		nextIDs[id] = struct{}{}
	}
	nextFingerprints := make(map[string]struct{}, len(parsed.RevokedFingerprints))
	for _, fp := range parsed.RevokedFingerprints {
		fp = strings.ToLower(strings.TrimSpace(fp))
		if fp == "" {
			continue
		}
		nextFingerprints[fp] = struct{}{}
	}

	globalRevocationList.mu.Lock()
	globalRevocationList.revokedKeyIDs = nextIDs
	globalRevocationList.revokedFingerprints = nextFingerprints
	globalRevocationList.mu.Unlock()
	log.Info().Str("path", path).Int("revoked_key_ids", len(nextIDs)).Int("revoked_fingerprints", len(nextFingerprints)).Msg("Loaded plugin signing CRL")
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
	globalRevocationList.revokedKeyIDs = next
	globalRevocationList.revokedFingerprints = make(map[string]struct{})
	globalRevocationList.mu.Unlock()
}

// resetRevokedFingerprintsForTest seeds the public-key fingerprint CRL.
func resetRevokedFingerprintsForTest(fingerprints ...string) {
	next := make(map[string]struct{}, len(fingerprints))
	for _, fp := range fingerprints {
		fp = strings.ToLower(strings.TrimSpace(fp))
		if fp == "" {
			continue
		}
		next[fp] = struct{}{}
	}
	globalRevocationList.mu.Lock()
	globalRevocationList.revokedFingerprints = next
	globalRevocationList.mu.Unlock()
}

// formatVerificationFailure builds a clear quarantine message tailored to
// why the signature did not verify. The publicKeyBytes argument is consulted
// for fingerprint-based revocation so operators see the actual cause rather
// than the generic "did not verify" fallback. publicKeyBytes may be empty
// when no signature was present at all.
func formatVerificationFailure(payload plugin_signing.SignedManifestPayload, publicKeyBytes []byte) string {
	// Fingerprint-CRL revocation is independent of the operator-declared
	// key_id label, so check it even when payload.KeyID is empty.
	if len(publicKeyBytes) > 0 && isRevokedPublicKey(publicKeyBytes) {
		fp := publicKeyFingerprint(publicKeyBytes)
		if payload.KeyID != "" {
			return fmt.Sprintf("plugin signing key fingerprint %s is revoked (key %q)", fp, payload.KeyID)
		}
		return fmt.Sprintf("plugin signing key fingerprint %s is revoked", fp)
	}
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

package plugin_signing

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	ManifestSignatureFile     = "manifest.sig"
	ManifestPublicKeyFile     = "manifest.pub"
	SignedManifestPayloadFile = "manifest.signed.json"
)

// SignedManifestPayload is the canonical wrapper that the signature covers.
// The bundle still ships manifest.json for human inspection, but the signed
// bytes are the canonical encoding of this struct (which embeds the manifest
// as RawMessage so its bytes round-trip exactly).
type SignedManifestPayload struct {
	Manifest  json.RawMessage `json:"manifest"`
	KeyID     string          `json:"key_id"`
	SignedAt  time.Time       `json:"signed_at"`
	ExpiresAt time.Time       `json:"expires_at"`
}

func CanonicalizeManifestPayload(manifestBytes []byte) ([]byte, error) {
	var payload interface{}

	decoder := json.NewDecoder(bytes.NewReader(manifestBytes))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return nil, fmt.Errorf("invalid manifest payload: %w", err)
	}

	var canonical bytes.Buffer
	if err := writeCanonicalJSON(&canonical, payload); err != nil {
		return nil, err
	}

	return canonical.Bytes(), nil
}

// BuildSignedPayload assembles the canonical signed payload bytes from the
// manifest plus signing metadata. The returned bytes are what the signer
// signs and what verifiers verify.
func BuildSignedPayload(manifestBytes []byte, keyID string, signedAt, expiresAt time.Time) ([]byte, error) {
	keyID = strings.TrimSpace(keyID)
	if keyID == "" {
		return nil, fmt.Errorf("key_id is required")
	}
	if signedAt.IsZero() {
		return nil, fmt.Errorf("signed_at is required")
	}
	if expiresAt.IsZero() {
		return nil, fmt.Errorf("expires_at is required")
	}
	if !expiresAt.After(signedAt) {
		return nil, fmt.Errorf("expires_at must be after signed_at")
	}

	canonicalManifest, err := CanonicalizeManifestPayload(manifestBytes)
	if err != nil {
		return nil, err
	}

	payload := SignedManifestPayload{
		Manifest:  canonicalManifest,
		KeyID:     keyID,
		SignedAt:  signedAt.UTC(),
		ExpiresAt: expiresAt.UTC(),
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal signed payload: %w", err)
	}
	return CanonicalizeManifestPayload(raw)
}

// ParseSignedPayload decodes the canonical bytes produced by BuildSignedPayload
// and returns both the parsed metadata and the canonical manifest bytes the
// payload commits to.
func ParseSignedPayload(signedPayloadBytes []byte) (SignedManifestPayload, []byte, error) {
	var payload SignedManifestPayload
	decoder := json.NewDecoder(bytes.NewReader(signedPayloadBytes))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return SignedManifestPayload{}, nil, fmt.Errorf("invalid signed payload: %w", err)
	}
	if strings.TrimSpace(payload.KeyID) == "" {
		return SignedManifestPayload{}, nil, fmt.Errorf("signed payload is missing key_id")
	}
	if payload.SignedAt.IsZero() {
		return SignedManifestPayload{}, nil, fmt.Errorf("signed payload is missing signed_at")
	}
	if payload.ExpiresAt.IsZero() {
		return SignedManifestPayload{}, nil, fmt.Errorf("signed payload is missing expires_at")
	}
	if !payload.ExpiresAt.After(payload.SignedAt) {
		return SignedManifestPayload{}, nil, fmt.Errorf("signed payload expires_at must be after signed_at")
	}
	canonicalManifest, err := CanonicalizeManifestPayload(payload.Manifest)
	if err != nil {
		return SignedManifestPayload{}, nil, fmt.Errorf("invalid embedded manifest: %w", err)
	}
	return payload, canonicalManifest, nil
}

// SignSignedPayload signs the canonical bytes of an already-built signed
// payload (see BuildSignedPayload) and returns the signature/public-key
// files in the same encoding the verifier expects.
func SignSignedPayload(signedPayloadBytes []byte, privateKey ed25519.PrivateKey) ([]byte, []byte, error) {
	if len(privateKey) != ed25519.PrivateKeySize {
		return nil, nil, fmt.Errorf("private key has %d bytes, want %d", len(privateKey), ed25519.PrivateKeySize)
	}

	canonical, err := CanonicalizeManifestPayload(signedPayloadBytes)
	if err != nil {
		return nil, nil, err
	}

	signature := ed25519.Sign(privateKey, canonical)
	publicKey := privateKey.Public().(ed25519.PublicKey)

	signatureFile, err := EncodeSignatureFile(signature)
	if err != nil {
		return nil, nil, err
	}
	publicKeyFile, err := EncodePublicKeyFile(publicKey)
	if err != nil {
		return nil, nil, err
	}

	return signatureFile, publicKeyFile, nil
}

// VerifySignedPayload confirms the signature covers the signed payload bytes,
// validates the embedded manifest matches the standalone manifest the bundle
// ships, and returns the parsed metadata for time/CRL checks by the caller.
func VerifySignedPayload(signedPayloadBytes, manifestBytes, signatureFileBytes, publicKeyFileBytes []byte) (SignedManifestPayload, bool, error) {
	canonicalSigned, err := CanonicalizeManifestPayload(signedPayloadBytes)
	if err != nil {
		return SignedManifestPayload{}, false, err
	}
	signature, err := DecodeSignatureFile(signatureFileBytes)
	if err != nil {
		return SignedManifestPayload{}, false, err
	}
	publicKey, err := DecodePublicKeyFile(publicKeyFileBytes)
	if err != nil {
		return SignedManifestPayload{}, false, err
	}
	if !ed25519.Verify(publicKey, canonicalSigned, signature) {
		return SignedManifestPayload{}, false, nil
	}

	payload, embeddedCanonicalManifest, err := ParseSignedPayload(signedPayloadBytes)
	if err != nil {
		return SignedManifestPayload{}, false, err
	}

	canonicalDisplayManifest, err := CanonicalizeManifestPayload(manifestBytes)
	if err != nil {
		return SignedManifestPayload{}, false, fmt.Errorf("invalid display manifest: %w", err)
	}
	if !bytes.Equal(embeddedCanonicalManifest, canonicalDisplayManifest) {
		return SignedManifestPayload{}, false, fmt.Errorf("signed payload manifest does not match bundle manifest.json")
	}

	return payload, true, nil
}

func EncodePublicKeyString(publicKey ed25519.PublicKey) (string, error) {
	if len(publicKey) != ed25519.PublicKeySize {
		return "", fmt.Errorf("public key has %d bytes, want %d", len(publicKey), ed25519.PublicKeySize)
	}

	return base64.StdEncoding.EncodeToString(publicKey), nil
}

func EncodePublicKeyFile(publicKey ed25519.PublicKey) ([]byte, error) {
	encoded, err := EncodePublicKeyString(publicKey)
	if err != nil {
		return nil, err
	}

	return []byte(encoded), nil
}

func DecodePublicKeyString(value string) (ed25519.PublicKey, error) {
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(value))
	if err != nil {
		return nil, fmt.Errorf("invalid plugin public key encoding: %w", err)
	}
	if len(decoded) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("plugin public key has %d bytes, want %d", len(decoded), ed25519.PublicKeySize)
	}

	return ed25519.PublicKey(decoded), nil
}

func DecodePublicKeyFile(data []byte) (ed25519.PublicKey, error) {
	return DecodePublicKeyString(string(data))
}

func EncodeSignatureFile(signature []byte) ([]byte, error) {
	if len(signature) != ed25519.SignatureSize {
		return nil, fmt.Errorf("signature has %d bytes, want %d", len(signature), ed25519.SignatureSize)
	}

	return []byte(base64.StdEncoding.EncodeToString(signature)), nil
}

func DecodeSignatureFile(data []byte) ([]byte, error) {
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(data)))
	if err != nil {
		return nil, fmt.Errorf("invalid plugin signature encoding: %w", err)
	}
	if len(decoded) != ed25519.SignatureSize {
		return nil, fmt.Errorf("plugin signature has %d bytes, want %d", len(decoded), ed25519.SignatureSize)
	}

	return decoded, nil
}

func writeCanonicalJSON(buffer *bytes.Buffer, value interface{}) error {
	switch typed := value.(type) {
	case nil:
		buffer.WriteString("null")
	case bool:
		if typed {
			buffer.WriteString("true")
		} else {
			buffer.WriteString("false")
		}
	case string:
		encoded, err := json.Marshal(typed)
		if err != nil {
			return fmt.Errorf("failed to encode canonical JSON string: %w", err)
		}
		buffer.Write(encoded)
	case json.Number:
		buffer.WriteString(typed.String())
	case float64:
		encoded, err := json.Marshal(typed)
		if err != nil {
			return fmt.Errorf("failed to encode canonical JSON number: %w", err)
		}
		buffer.Write(encoded)
	case []interface{}:
		buffer.WriteByte('[')
		for index, item := range typed {
			if index > 0 {
				buffer.WriteByte(',')
			}
			if err := writeCanonicalJSON(buffer, item); err != nil {
				return err
			}
		}
		buffer.WriteByte(']')
	case map[string]interface{}:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		buffer.WriteByte('{')
		for index, key := range keys {
			if index > 0 {
				buffer.WriteByte(',')
			}

			encodedKey, err := json.Marshal(key)
			if err != nil {
				return fmt.Errorf("failed to encode canonical JSON object key: %w", err)
			}
			buffer.Write(encodedKey)
			buffer.WriteByte(':')

			if err := writeCanonicalJSON(buffer, typed[key]); err != nil {
				return err
			}
		}
		buffer.WriteByte('}')
	default:
		encoded, err := json.Marshal(typed)
		if err != nil {
			return fmt.Errorf("failed to encode canonical JSON value: %w", err)
		}
		buffer.Write(encoded)
	}

	return nil
}

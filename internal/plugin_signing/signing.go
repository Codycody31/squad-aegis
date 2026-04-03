package plugin_signing

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

const (
	ManifestSignatureFile = "manifest.sig"
	ManifestPublicKeyFile = "manifest.pub"
)

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

func SignManifest(manifestBytes []byte, privateKey ed25519.PrivateKey) ([]byte, []byte, error) {
	canonicalManifest, err := CanonicalizeManifestPayload(manifestBytes)
	if err != nil {
		return nil, nil, err
	}

	if len(privateKey) != ed25519.PrivateKeySize {
		return nil, nil, fmt.Errorf("private key has %d bytes, want %d", len(privateKey), ed25519.PrivateKeySize)
	}

	signature := ed25519.Sign(privateKey, canonicalManifest)
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

func VerifyManifest(manifestBytes, signatureFileBytes, publicKeyFileBytes []byte) (bool, error) {
	canonicalManifest, err := CanonicalizeManifestPayload(manifestBytes)
	if err != nil {
		return false, err
	}

	signature, err := DecodeSignatureFile(signatureFileBytes)
	if err != nil {
		return false, err
	}
	publicKey, err := DecodePublicKeyFile(publicKeyFileBytes)
	if err != nil {
		return false, err
	}

	return ed25519.Verify(publicKey, canonicalManifest, signature), nil
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

package plugin_loader

import (
	"crypto/ed25519"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// SignatureAlgorithm represents the cryptographic algorithm used for signatures
type SignatureAlgorithm string

const (
	SignatureAlgorithmED25519 SignatureAlgorithm = "ed25519"
)

// PublicKey represents a trusted public key for signature verification
type PublicKey struct {
	ID        uuid.UUID
	KeyName   string
	PublicKey []byte
	Algorithm SignatureAlgorithm
	AddedBy   uuid.UUID
	Revoked   bool
}

// SignatureVerifier handles plugin signature verification
type SignatureVerifier struct {
	db          *sql.DB
	publicKeys  map[string]*PublicKey // keyName -> PublicKey
	mu          sync.RWMutex
}

// NewSignatureVerifier creates a new signature verifier
func NewSignatureVerifier(db *sql.DB) (*SignatureVerifier, error) {
	sv := &SignatureVerifier{
		db:         db,
		publicKeys: make(map[string]*PublicKey),
	}
	
	// Load public keys from database
	if err := sv.loadPublicKeys(); err != nil {
		return nil, fmt.Errorf("failed to load public keys: %w", err)
	}
	
	return sv, nil
}

// VerifySignature verifies a plugin's signature against trusted public keys
func (sv *SignatureVerifier) VerifySignature(pluginBytes []byte, signature []byte) error {
	if len(signature) == 0 {
		return fmt.Errorf("signature is empty")
	}
	
	// Calculate hash of plugin bytes
	hash := sha256.Sum256(pluginBytes)
	
	// Try to verify with each active public key
	sv.mu.RLock()
	defer sv.mu.RUnlock()
	
	if len(sv.publicKeys) == 0 {
		return fmt.Errorf("no trusted public keys available for verification")
	}
	
	var lastErr error
	for keyName, pubKey := range sv.publicKeys {
		if pubKey.Revoked {
			continue
		}
		
		switch pubKey.Algorithm {
		case SignatureAlgorithmED25519:
			if len(pubKey.PublicKey) != ed25519.PublicKeySize {
				log.Warn().Str("key_name", keyName).Msg("Invalid ED25519 public key size")
				continue
			}
			
			if ed25519.Verify(ed25519.PublicKey(pubKey.PublicKey), hash[:], signature) {
				log.Info().Str("key_name", keyName).Msg("Signature verified successfully")
				return nil
			}
			lastErr = fmt.Errorf("signature verification failed with key %s", keyName)
			
		default:
			log.Warn().
				Str("key_name", keyName).
				Str("algorithm", string(pubKey.Algorithm)).
				Msg("Unsupported signature algorithm")
		}
	}
	
	if lastErr != nil {
		return lastErr
	}
	
	return fmt.Errorf("signature verification failed with all trusted keys")
}

// AddPublicKey adds a new trusted public key
func (sv *SignatureVerifier) AddPublicKey(keyName string, publicKey []byte, algorithm SignatureAlgorithm, addedBy uuid.UUID) error {
	sv.mu.Lock()
	defer sv.mu.Unlock()
	
	// Validate algorithm
	if algorithm != SignatureAlgorithmED25519 {
		return fmt.Errorf("unsupported signature algorithm: %s", algorithm)
	}
	
	// Validate key size
	if algorithm == SignatureAlgorithmED25519 && len(publicKey) != ed25519.PublicKeySize {
		return fmt.Errorf("invalid ED25519 public key size: expected %d, got %d", 
			ed25519.PublicKeySize, len(publicKey))
	}
	
	// Check if key already exists
	if _, exists := sv.publicKeys[keyName]; exists {
		return fmt.Errorf("public key with name %s already exists", keyName)
	}
	
	// Insert into database
	id := uuid.New()
	query := `
		INSERT INTO plugin_public_keys (id, key_name, public_key, algorithm, added_by, revoked)
		VALUES ($1, $2, $3, $4, $5, false)
	`
	
	if _, err := sv.db.Exec(query, id, keyName, publicKey, string(algorithm), addedBy); err != nil {
		return fmt.Errorf("failed to insert public key into database: %w", err)
	}
	
	// Add to memory
	sv.publicKeys[keyName] = &PublicKey{
		ID:        id,
		KeyName:   keyName,
		PublicKey: publicKey,
		Algorithm: algorithm,
		AddedBy:   addedBy,
		Revoked:   false,
	}
	
	log.Info().
		Str("key_name", keyName).
		Str("algorithm", string(algorithm)).
		Msg("Added new trusted public key")
	
	return nil
}

// RevokePublicKey revokes a trusted public key
func (sv *SignatureVerifier) RevokePublicKey(keyName string) error {
	sv.mu.Lock()
	defer sv.mu.Unlock()
	
	pubKey, exists := sv.publicKeys[keyName]
	if !exists {
		return fmt.Errorf("public key %s not found", keyName)
	}
	
	// Update database
	query := `UPDATE plugin_public_keys SET revoked = true WHERE key_name = $1`
	if _, err := sv.db.Exec(query, keyName); err != nil {
		return fmt.Errorf("failed to revoke public key in database: %w", err)
	}
	
	// Update in memory
	pubKey.Revoked = true
	
	log.Info().Str("key_name", keyName).Msg("Revoked public key")
	
	return nil
}

// ListPublicKeys returns all public keys (including revoked)
func (sv *SignatureVerifier) ListPublicKeys() []*PublicKey {
	sv.mu.RLock()
	defer sv.mu.RUnlock()
	
	keys := make([]*PublicKey, 0, len(sv.publicKeys))
	for _, key := range sv.publicKeys {
		keys = append(keys, key)
	}
	
	return keys
}

// GetPublicKey returns a specific public key by name
func (sv *SignatureVerifier) GetPublicKey(keyName string) (*PublicKey, error) {
	sv.mu.RLock()
	defer sv.mu.RUnlock()
	
	key, exists := sv.publicKeys[keyName]
	if !exists {
		return nil, fmt.Errorf("public key %s not found", keyName)
	}
	
	return key, nil
}

// loadPublicKeys loads all public keys from the database
func (sv *SignatureVerifier) loadPublicKeys() error {
	query := `
		SELECT id, key_name, public_key, algorithm, added_by, revoked
		FROM plugin_public_keys
		ORDER BY added_at ASC
	`
	
	rows, err := sv.db.Query(query)
	if err != nil {
		// If table doesn't exist yet, that's okay
		if err == sql.ErrNoRows {
			return nil
		}
		return fmt.Errorf("failed to query public keys: %w", err)
	}
	defer rows.Close()
	
	for rows.Next() {
		var key PublicKey
		var algorithm string
		
		if err := rows.Scan(&key.ID, &key.KeyName, &key.PublicKey, &algorithm, &key.AddedBy, &key.Revoked); err != nil {
			return fmt.Errorf("failed to scan public key row: %w", err)
		}
		
		key.Algorithm = SignatureAlgorithm(algorithm)
		sv.publicKeys[key.KeyName] = &key
	}
	
	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating public key rows: %w", err)
	}
	
	log.Info().Int("count", len(sv.publicKeys)).Msg("Loaded trusted public keys")
	
	return nil
}

// ReloadPublicKeys reloads public keys from the database
func (sv *SignatureVerifier) ReloadPublicKeys() error {
	sv.mu.Lock()
	defer sv.mu.Unlock()
	
	sv.publicKeys = make(map[string]*PublicKey)
	return sv.loadPublicKeys()
}

// SignPlugin signs a plugin file (utility function for plugin developers)
func SignPlugin(pluginBytes []byte, privateKey ed25519.PrivateKey) ([]byte, error) {
	if len(privateKey) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("invalid private key size: expected %d, got %d", 
			ed25519.PrivateKeySize, len(privateKey))
	}
	
	// Calculate hash of plugin bytes
	hash := sha256.Sum256(pluginBytes)
	
	// Sign the hash
	signature := ed25519.Sign(privateKey, hash[:])
	
	return signature, nil
}

// GenerateKeyPair generates a new ED25519 key pair (utility function)
func GenerateKeyPair() (publicKey ed25519.PublicKey, privateKey ed25519.PrivateKey, err error) {
	publicKey, privateKey, err = ed25519.GenerateKey(nil)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate key pair: %w", err)
	}
	
	return publicKey, privateKey, nil
}

// EncodePublicKey encodes a public key to base64 for storage/display
func EncodePublicKey(publicKey ed25519.PublicKey) string {
	return base64.StdEncoding.EncodeToString(publicKey)
}

// DecodePublicKey decodes a base64-encoded public key
func DecodePublicKey(encoded string) (ed25519.PublicKey, error) {
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("failed to decode public key: %w", err)
	}
	
	if len(decoded) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid public key size: expected %d, got %d", 
			ed25519.PublicKeySize, len(decoded))
	}
	
	return ed25519.PublicKey(decoded), nil
}


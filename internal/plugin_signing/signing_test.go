package plugin_signing

import (
	"crypto/ed25519"
	"crypto/rand"
	"strings"
	"testing"
	"time"
)

func TestCanonicalizeManifestPayloadSortsObjectKeys(t *testing.T) {
	t.Parallel()

	raw := []byte(`{
		"version":"1.0.0",
		"plugin_id":"example.plugin",
		"nested":{"b":2,"a":1},
		"items":[{"z":true,"a":false},"hello"]
	}`)

	canonical, err := CanonicalizeManifestPayload(raw)
	if err != nil {
		t.Fatalf("CanonicalizeManifestPayload() error = %v", err)
	}

	const want = `{"items":[{"a":false,"z":true},"hello"],"nested":{"a":1,"b":2},"plugin_id":"example.plugin","version":"1.0.0"}`
	if got := string(canonical); got != want {
		t.Fatalf("CanonicalizeManifestPayload() = %q, want %q", got, want)
	}
}

func TestSignAndVerifySignedPayloadHappyPath(t *testing.T) {
	t.Parallel()

	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("ed25519.GenerateKey() error = %v", err)
	}

	manifest := []byte(`{"version":"1.0.0","plugin_id":"example.plugin","nested":{"b":2,"a":1}}`)
	signedAt := time.Now().UTC()
	expiresAt := signedAt.Add(24 * time.Hour)

	signedPayload, err := BuildSignedPayload(manifest, "ops-key-2026-q1", signedAt, expiresAt)
	if err != nil {
		t.Fatalf("BuildSignedPayload() error = %v", err)
	}
	signature, publicKey, err := SignSignedPayload(signedPayload, privateKey)
	if err != nil {
		t.Fatalf("SignSignedPayload() error = %v", err)
	}

	reorderedManifest := []byte(`{"nested":{"a":1,"b":2},"plugin_id":"example.plugin","version":"1.0.0"}`)
	payload, ok, err := VerifySignedPayload(signedPayload, reorderedManifest, signature, publicKey)
	if err != nil {
		t.Fatalf("VerifySignedPayload() error = %v", err)
	}
	if !ok {
		t.Fatal("VerifySignedPayload() = false, want true")
	}
	if got, want := payload.KeyID, "ops-key-2026-q1"; got != want {
		t.Fatalf("payload.KeyID = %q, want %q", got, want)
	}
	if !payload.ExpiresAt.Equal(expiresAt) {
		t.Fatalf("payload.ExpiresAt = %v, want %v", payload.ExpiresAt, expiresAt)
	}
}

func TestVerifySignedPayloadRejectsTamperedManifest(t *testing.T) {
	t.Parallel()

	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("ed25519.GenerateKey() error = %v", err)
	}

	manifest := []byte(`{"plugin_id":"example.plugin","version":"1.0.0"}`)
	signedAt := time.Now().UTC()
	signedPayload, err := BuildSignedPayload(manifest, "ops-key", signedAt, signedAt.Add(time.Hour))
	if err != nil {
		t.Fatalf("BuildSignedPayload() error = %v", err)
	}
	signature, publicKey, err := SignSignedPayload(signedPayload, privateKey)
	if err != nil {
		t.Fatalf("SignSignedPayload() error = %v", err)
	}

	tampered := []byte(`{"plugin_id":"example.evil","version":"1.0.0"}`)
	_, ok, err := VerifySignedPayload(signedPayload, tampered, signature, publicKey)
	if err == nil {
		t.Fatal("VerifySignedPayload() with tampered manifest error = nil, want error")
	}
	if ok {
		t.Fatal("VerifySignedPayload() with tampered manifest ok = true, want false")
	}
}

func TestBuildSignedPayloadRejectsMissingKeyID(t *testing.T) {
	t.Parallel()

	manifest := []byte(`{"plugin_id":"example.plugin","version":"1.0.0"}`)
	signedAt := time.Now().UTC()
	if _, err := BuildSignedPayload(manifest, "", signedAt, signedAt.Add(time.Hour)); err == nil {
		t.Fatal("BuildSignedPayload() with empty key_id error = nil, want error")
	}
}

func TestBuildSignedPayloadRejectsExpiryBeforeSignedAt(t *testing.T) {
	t.Parallel()

	manifest := []byte(`{"plugin_id":"example.plugin","version":"1.0.0"}`)
	signedAt := time.Now().UTC()
	if _, err := BuildSignedPayload(manifest, "ops", signedAt, signedAt.Add(-time.Hour)); err == nil {
		t.Fatal("BuildSignedPayload() with expires_at before signed_at error = nil, want error")
	}
}

func TestParseSignedPayloadRejectsMissingFields(t *testing.T) {
	t.Parallel()

	missingKeyID := []byte(`{"manifest":{},"signed_at":"2026-01-01T00:00:00Z","expires_at":"2026-01-02T00:00:00Z"}`)
	if _, _, err := ParseSignedPayload(missingKeyID); err == nil || !strings.Contains(err.Error(), "key_id") {
		t.Fatalf("ParseSignedPayload() error = %v, want missing key_id", err)
	}
}

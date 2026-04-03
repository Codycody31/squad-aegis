package plugin_signing

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"
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

func TestSignAndVerifyManifestUsesCanonicalPayload(t *testing.T) {
	t.Parallel()

	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("ed25519.GenerateKey() error = %v", err)
	}

	raw := []byte(`{"version":"1.0.0","plugin_id":"example.plugin","nested":{"b":2,"a":1}}`)
	signatureFile, publicKeyFile, err := SignManifest(raw, privateKey)
	if err != nil {
		t.Fatalf("SignManifest() error = %v", err)
	}

	reordered := []byte(`{"nested":{"a":1,"b":2},"plugin_id":"example.plugin","version":"1.0.0"}`)
	ok, err := VerifyManifest(reordered, signatureFile, publicKeyFile)
	if err != nil {
		t.Fatalf("VerifyManifest() error = %v", err)
	}
	if !ok {
		t.Fatal("VerifyManifest() = false, want true")
	}
}

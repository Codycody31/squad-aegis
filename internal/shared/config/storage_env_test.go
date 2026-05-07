package config

import (
	"testing"

	"github.com/cristalhq/aconfig"
	"github.com/cristalhq/aconfig/aconfigyaml"
)

// Sanity check that natural env var names reach the doubly-nested
// Storage.S3.* fields. Without explicit struct tags pinning the parent
// path, aconfig splits "S3" into two words ("S","3") and the env var
// becomes STORAGE_S_3_BUCKET — which no operator would guess.
func TestStorageS3EnvVarsLoad(t *testing.T) {
	t.Setenv("STORAGE_TYPE", "s3")
	t.Setenv("STORAGE_S3_BUCKET", "real-bucket")
	t.Setenv("STORAGE_S3_ACCESS_KEY_ID", "AKIAEXAMPLE")
	t.Setenv("STORAGE_S3_SECRET_ACCESS_KEY", "supersecret")
	t.Setenv("STORAGE_S3_ENDPOINT", "minio.example.com")
	t.Setenv("STORAGE_S3_REGION", "eu-west-1")
	t.Setenv("STORAGE_S3_USE_SSL", "false")

	var cfg Struct
	loader := aconfig.LoaderFor(&cfg, aconfig.Config{
		SkipFlags:        true,
		AllowUnknownEnvs: true,
		FileDecoders: map[string]aconfig.FileDecoder{
			".yaml": aconfigyaml.New(),
		},
	})
	if err := loader.Load(); err != nil {
		t.Fatalf("loader.Load: %v", err)
	}

	checks := []struct {
		name string
		got  any
		want any
	}{
		{"Storage.Type", cfg.Storage.Type, "s3"},
		{"Storage.S3.Bucket", cfg.Storage.S3.Bucket, "real-bucket"},
		{"Storage.S3.AccessKeyID", cfg.Storage.S3.AccessKeyID, "AKIAEXAMPLE"},
		{"Storage.S3.SecretAccessKey", cfg.Storage.S3.SecretAccessKey, "supersecret"},
		{"Storage.S3.Endpoint", cfg.Storage.S3.Endpoint, "minio.example.com"},
		{"Storage.S3.Region", cfg.Storage.S3.Region, "eu-west-1"},
		{"Storage.S3.UseSSL", cfg.Storage.S3.UseSSL, false},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %v, want %v", c.name, c.got, c.want)
		}
	}
}

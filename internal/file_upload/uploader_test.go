package file_upload

import (
	"context"
	"path/filepath"
	"testing"
)

func TestValidateFilePath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name:    "accepts unix absolute path",
			path:    "/opt/squad/ServerConfig/Bans.cfg",
			wantErr: false,
		},
		{
			name:    "accepts windows absolute path with forward slashes",
			path:    "D:/SquadGame/ServerConfig/Bans.cfg",
			wantErr: false,
		},
		{
			name:    "accepts windows absolute path with backslashes",
			path:    "D:\\SquadGame\\ServerConfig\\Bans.cfg",
			wantErr: false,
		},
		{
			name:    "rejects relative path",
			path:    "SquadGame/ServerConfig/Bans.cfg",
			wantErr: true,
		},
		{
			name:    "rejects traversal",
			path:    "/opt/squad/../etc/passwd",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateFilePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateFilePath(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
			}
		})
	}
}

func TestLocalUploaderUploadReadAndConnection(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	targetPath := filepath.Join(dir, "MOTD.cfg")
	content := "Welcome to the server"

	uploader, err := NewUploader(UploadConfig{
		Protocol: "local",
		FilePath: targetPath,
	})
	if err != nil {
		t.Fatalf("NewUploader returned error: %v", err)
	}
	defer uploader.Close()

	if err := uploader.TestConnection(context.Background()); err != nil {
		t.Fatalf("TestConnection returned error: %v", err)
	}

	if err := uploader.Upload(context.Background(), content); err != nil {
		t.Fatalf("Upload returned error: %v", err)
	}

	got, err := uploader.Read(context.Background())
	if err != nil {
		t.Fatalf("Read returned error: %v", err)
	}
	if got != content {
		t.Fatalf("Read returned %q, want %q", got, content)
	}
}

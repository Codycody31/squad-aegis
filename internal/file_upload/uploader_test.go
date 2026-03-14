package file_upload

import "testing"

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
			path:    `D:\\SquadGame\\ServerConfig\\Bans.cfg`,
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

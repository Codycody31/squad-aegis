package core

import "testing"

func TestValidatePasswordPolicy(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  error
	}{
		{
			name:     "accepts common symbols outside the old allowlist",
			password: "Valid1#x",
		},
		{
			name:     "rejects too short passwords",
			password: "Aa1#abc",
			wantErr:  ErrPasswordTooShort,
		},
		{
			name:     "rejects missing uppercase",
			password: "valid1#x",
			wantErr:  ErrPasswordPolicy,
		},
		{
			name:     "rejects missing lowercase",
			password: "VALID1#X",
			wantErr:  ErrPasswordPolicy,
		},
		{
			name:     "rejects missing number",
			password: "ValidPwd#",
			wantErr:  ErrPasswordPolicy,
		},
		{
			name:     "rejects missing special character",
			password: "Valid123",
			wantErr:  ErrPasswordPolicy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePasswordPolicy(tt.password)
			if err != tt.wantErr {
				t.Fatalf("ValidatePasswordPolicy(%q) error = %v, want %v", tt.password, err, tt.wantErr)
			}
		})
	}
}

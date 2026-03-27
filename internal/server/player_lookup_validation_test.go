package server

import "testing"

func TestNormalizeOptionalEOSID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		input          string
		wantNormalized string
		wantProvided   bool
		wantValid      bool
	}{
		{
			name:         "blank input is absent",
			input:        "   ",
			wantProvided: false,
			wantValid:    true,
		},
		{
			name:           "valid EOS ID is normalized",
			input:          "  ABCDEF0123456789ABCDEF0123456789  ",
			wantNormalized: "abcdef0123456789abcdef0123456789",
			wantProvided:   true,
			wantValid:      true,
		},
		{
			name:         "malformed EOS ID is flagged",
			input:        "not-an-eos-id",
			wantProvided: true,
			wantValid:    false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotNormalized, gotProvided, gotValid := normalizeOptionalEOSID(tt.input)
			if gotNormalized != tt.wantNormalized {
				t.Fatalf("normalizeOptionalEOSID() normalized = %q, want %q", gotNormalized, tt.wantNormalized)
			}
			if gotProvided != tt.wantProvided {
				t.Fatalf("normalizeOptionalEOSID() provided = %v, want %v", gotProvided, tt.wantProvided)
			}
			if gotValid != tt.wantValid {
				t.Fatalf("normalizeOptionalEOSID() valid = %v, want %v", gotValid, tt.wantValid)
			}
		})
	}
}

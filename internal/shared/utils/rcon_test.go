package utils

import "testing"

func TestSanitizeRCONParam(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "normal string passes through unchanged",
			input: "hello world",
			want:  "hello world",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "newline injection",
			input: "line1\nline2",
			want:  "line1line2",
		},
		{
			name:  "carriage return injection",
			input: "line1\rline2",
			want:  "line1line2",
		},
		{
			name:  "CRLF injection",
			input: "line1\r\nline2",
			want:  "line1line2",
		},
		{
			name:  "double quote injection",
			input: `say "hello"`,
			want:  "say hello",
		},
		{
			name:  "null byte injection",
			input: "before\x00after",
			want:  "beforeafter",
		},
		{
			name:  "mixed injection with all dangerous chars",
			input: "start\"\n\r\x00end",
			want:  "startend",
		},
		{
			name:  "string with only dangerous characters returns empty",
			input: "\"\n\r\x00",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeRCONParam(tt.input)
			if got != tt.want {
				t.Errorf("SanitizeRCONParam(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSanitizeAndQuoteRCONParam(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "normal string is quoted",
			input: "hello world",
			want:  `"hello world"`,
		},
		{
			name:  "empty string produces empty quotes",
			input: "",
			want:  `""`,
		},
		{
			name:  "dangerous chars stripped then quoted",
			input: "say\"\nhello\r\x00world",
			want:  `"sayhelloworld"`,
		},
		{
			name:  "only dangerous chars produces empty quotes",
			input: "\"\n\r\x00",
			want:  `""`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeAndQuoteRCONParam(tt.input)
			if got != tt.want {
				t.Errorf("SanitizeAndQuoteRCONParam(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

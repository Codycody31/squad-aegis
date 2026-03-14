package utils

import "testing"

func TestNormalizeEOSID(t *testing.T) {
	input := "  ABCDEF0123456789ABCDEF0123456789  "
	got := NormalizeEOSID(input)
	want := "abcdef0123456789abcdef0123456789"

	if got != want {
		t.Fatalf("NormalizeEOSID(%q) = %q, want %q", input, got, want)
	}

	if !IsEOSID(got) {
		t.Fatalf("normalized EOS ID %q should validate", got)
	}
}

package utils

import (
	"net"
	"testing"
)

func parseIPHelper(s string) net.IP {
	return net.ParseIP(s)
}

func TestValidateRemoteURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{
			name:    "valid https URL",
			url:     "https://example.com/bans.cfg",
			wantErr: false,
		},
		{
			name:    "valid http URL",
			url:     "http://example.com/bans.cfg",
			wantErr: false,
		},
		{
			name:    "file scheme rejected",
			url:     "file:///etc/passwd",
			wantErr: true,
		},
		{
			name:    "ftp scheme rejected",
			url:     "ftp://example.com/bans.cfg",
			wantErr: true,
		},
		{
			name:    "empty scheme rejected",
			url:     "://example.com",
			wantErr: true,
		},
		{
			name:    "localhost rejected",
			url:     "http://localhost/admin",
			wantErr: true,
		},
		{
			name:    "127.0.0.1 rejected",
			url:     "http://127.0.0.1/admin",
			wantErr: true,
		},
		{
			name:    "private 10.x rejected",
			url:     "http://10.0.0.1/admin",
			wantErr: true,
		},
		{
			name:    "private 172.16.x rejected",
			url:     "http://172.16.0.1/admin",
			wantErr: true,
		},
		{
			name:    "private 192.168.x rejected",
			url:     "http://192.168.1.1/admin",
			wantErr: true,
		},
		{
			name:    "link-local 169.254.x rejected",
			url:     "http://169.254.169.254/latest/meta-data/",
			wantErr: true,
		},
		{
			name:    "IPv6 loopback rejected",
			url:     "http://[::1]/admin",
			wantErr: true,
		},
		{
			name:    "no hostname rejected",
			url:     "http:///path",
			wantErr: true,
		},
		{
			name:    "unresolvable hostname rejected",
			url:     "http://this-domain-does-not-exist-abc123xyz.example/bans",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRemoteURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateRemoteURL(%q) error = %v, wantErr %v", tt.url, err, tt.wantErr)
			}
		})
	}
}

func TestIsPrivateIP(t *testing.T) {
	tests := []struct {
		ip      string
		private bool
	}{
		{"10.0.0.1", true},
		{"10.255.255.255", true},
		{"172.16.0.1", true},
		{"172.31.255.255", true},
		{"172.32.0.1", false},
		{"192.168.0.1", true},
		{"192.168.255.255", true},
		{"127.0.0.1", true},
		{"127.255.255.255", true},
		{"169.254.169.254", true},
		{"8.8.8.8", false},
		{"1.1.1.1", false},
	}

	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			ip := parseIPHelper(tt.ip)
			if ip == nil {
				t.Fatalf("failed to parse IP %s", tt.ip)
			}
			got := IsPrivateIP(ip)
			if got != tt.private {
				t.Errorf("IsPrivateIP(%s) = %v, want %v", tt.ip, got, tt.private)
			}
		})
	}
}

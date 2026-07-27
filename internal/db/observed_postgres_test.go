package db

import "testing"

func TestPostgresQueryLabel(t *testing.T) {
	tests := []struct {
		query string
		want  string
	}{
		{"/* sa.audit.list */ SELECT 1", "sa.audit.list"},
		{"WITH data AS (SELECT 1) /* sa.metrics.summary */ SELECT * FROM data", "sa.metrics.summary"},
		{"SELECT 1", "unlabeled"},
		{"/* migration */ SELECT 1", "unlabeled"},
	}
	for _, tt := range tests {
		if got := postgresQueryLabel(tt.query); got != tt.want {
			t.Errorf("postgresQueryLabel(%q) = %q, want %q", tt.query, got, tt.want)
		}
	}
}

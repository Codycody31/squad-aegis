package clickhouse

import "testing"

func TestQueryLabel(t *testing.T) {
	tests := []struct {
		query string
		want  string
	}{
		{"/* sa.audit.list */ SELECT 1", "sa.audit.list"},
		{"\n\t/* sa.player.combat */\nSELECT 1", "sa.player.combat"},
		{"WITH ids AS (SELECT 1) /* sa.player.profile */ SELECT * FROM ids", "sa.player.profile"},
		{"SELECT 1", "unlabeled"},
		{"/* */ SELECT 1", "unlabeled"},
		{"/* migration note */ SELECT 1", "unlabeled"},
		{"/* unterminated", "unlabeled"},
	}
	for _, tt := range tests {
		if got := queryLabel(tt.query); got != tt.want {
			t.Errorf("queryLabel(%q) = %q, want %q", tt.query, got, tt.want)
		}
	}
}

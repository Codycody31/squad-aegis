package plugin_manager

import "testing"

func TestHasMultipleSQLStatements(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		query string
		want  bool
	}{
		{
			name:  "allows single select without semicolon",
			query: "SELECT 1",
			want:  false,
		},
		{
			name:  "allows trailing semicolon",
			query: "SELECT 1;",
			want:  false,
		},
		{
			name:  "allows trailing semicolon with comment",
			query: "SELECT 1; -- keep this query readable",
			want:  false,
		},
		{
			name:  "allows semicolon inside string literal",
			query: "SELECT ';' AS separator;",
			want:  false,
		},
		{
			name:  "allows semicolon inside quoted identifier",
			query: `SELECT "semi;colon" FROM "table";`,
			want:  false,
		},
		{
			name:  "allows semicolon inside block comment",
			query: "SELECT 1 /* ; */;",
			want:  false,
		},
		{
			name:  "allows semicolon inside dollar-quoted string",
			query: "SELECT $$;$$;",
			want:  false,
		},
		{
			name:  "rejects second statement",
			query: "SELECT 1; SELECT 2",
			want:  true,
		},
		{
			name:  "rejects second statement after comment",
			query: "SELECT 1; -- done\nSELECT 2",
			want:  true,
		},
		{
			name:  "rejects duplicate top-level semicolons",
			query: "SELECT 1;;",
			want:  true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := hasMultipleSQLStatements(tt.query); got != tt.want {
				t.Fatalf("hasMultipleSQLStatements(%q) = %v, want %v", tt.query, got, tt.want)
			}
		})
	}
}

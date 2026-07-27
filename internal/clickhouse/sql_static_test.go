package clickhouse

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestRuntimeGoSQLDoesNotUseOffset(t *testing.T) {
	repositoryRoot := filepath.Clean(filepath.Join("..", ".."))
	offsetPattern := regexp.MustCompile(`\bOFF` + `SET\b`)

	var matches []string
	err := filepath.WalkDir(repositoryRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == ".git" || entry.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if offsetPattern.Match(body) {
			matches = append(matches, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("runtime Go SQL contains offset pagination: %s", strings.Join(matches, ", "))
	}
}

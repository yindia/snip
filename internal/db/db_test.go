package db

import (
	"os"
	"testing"
)

func TestOpenCreatesSchema(t *testing.T) {
	dir := t.TempDir()
	sqlDB, path, err := Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer sqlDB.Close()

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected db file at %s: %v", path, err)
	}

	rows, err := sqlDB.Query(`SELECT name FROM sqlite_master WHERE type = 'table'`)
	if err != nil {
		t.Fatalf("query tables: %v", err)
	}
	defer rows.Close()

	seen := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan: %v", err)
		}
		seen[name] = true
	}
	for _, table := range []string{"collections", "documents", "documents_fts", "content_vectors", "path_contexts"} {
		if !seen[table] {
			t.Fatalf("expected table %q", table)
		}
	}
}

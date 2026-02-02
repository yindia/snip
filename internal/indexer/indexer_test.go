package indexer

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"snip/internal/db"
)

func TestUpdateRespectsExtensionsAndGitignore(t *testing.T) {
	work := t.TempDir()
	colDir := filepath.Join(work, "repo")
	if err := os.MkdirAll(filepath.Join(colDir, "sub"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(colDir, "a.md"), []byte("# A"), 0o644); err != nil {
		t.Fatalf("write a.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(colDir, "b.txt"), []byte("ignore"), 0o644); err != nil {
		t.Fatalf("write b.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(colDir, "ignored.md"), []byte("# Ignored"), 0o644); err != nil {
		t.Fatalf("write ignored.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(colDir, "sub", "c.md"), []byte("# C"), 0o644); err != nil {
		t.Fatalf("write c.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(colDir, ".gitignore"), []byte("ignored.md\nsub/\n"), 0o644); err != nil {
		t.Fatalf("write gitignore: %v", err)
	}

	sqlDB, _, err := db.Open(filepath.Join(work, "index"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer sqlDB.Close()
	if _, err := sqlDB.Exec(`INSERT INTO collections(name, path, mask) VALUES(?,?,?)`, "repo", colDir, "md"); err != nil {
		t.Fatalf("insert collection: %v", err)
	}

	stats, err := Update(context.Background(), sqlDB, []Collection{{Name: "repo", Path: colDir, Extensions: "md"}})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if stats.Updated != 1 || stats.Documents != 1 {
		t.Fatalf("expected 1 updated document, got updated=%d documents=%d", stats.Updated, stats.Documents)
	}

	var count int
	if err := sqlDB.QueryRow(`SELECT COUNT(*) FROM documents`).Scan(&count); err != nil {
		t.Fatalf("count documents: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 document in db, got %d", count)
	}
}

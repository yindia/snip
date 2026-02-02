package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"snip/internal/db"
	"snip/internal/indexer"
)

func TestUpdateRemovesDeletedFile(t *testing.T) {
	workDir := t.TempDir()
	colDir := filepath.Join(workDir, "notes")
	if err := os.MkdirAll(colDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(colDir, "doc.md")
	if err := os.WriteFile(path, []byte("# Title\n\ntext"), 0o644); err != nil {
		t.Fatal(err)
	}
	indexDir := filepath.Join(workDir, "index")
	sqlDB, _, err := db.Open(indexDir)
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	if _, err := sqlDB.Exec(`INSERT INTO collections(name, path, mask) VALUES(?,?,?)`, "notes", colDir, "md"); err != nil {
		t.Fatal(err)
	}
	if _, err := indexer.Update(context.Background(), sqlDB, []indexer.Collection{{Name: "notes", Path: colDir, Extensions: "md"}}); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	stats, err := indexer.Update(context.Background(), sqlDB, []indexer.Collection{{Name: "notes", Path: colDir, Extensions: "md"}})
	if err != nil {
		t.Fatal(err)
	}
	if stats.Removed == 0 {
		t.Fatalf("expected removed document")
	}
}

package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"snip/internal/db"
	"snip/internal/indexer"
	"snip/internal/search"
)

func TestUpdateAndSearch(t *testing.T) {
	workDir := t.TempDir()
	colDir := filepath.Join(workDir, "notes")
	if err := os.MkdirAll(colDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "# Distributed Systems\n\nThis is a test document."
	if err := os.WriteFile(filepath.Join(colDir, "doc.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	indexDir := filepath.Join(workDir, "index")
	sqlDB, _, err := db.Open(indexDir)
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	if _, err := sqlDB.Exec(`INSERT INTO collections(name, path, mask) VALUES(?,?,?)`, "notes", colDir, ""); err != nil {
		t.Fatal(err)
	}
	stats, err := indexer.Update(context.Background(), sqlDB, []indexer.Collection{{Name: "notes", Path: colDir, Extensions: ""}})
	if err != nil {
		t.Fatal(err)
	}
	if stats.Updated == 0 {
		t.Fatalf("expected indexed document")
	}
	results, err := search.FTSSearch(context.Background(), sqlDB, "distributed", search.Options{Limit: 5, All: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatalf("expected search results")
	}
}

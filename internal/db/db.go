package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

func Open(indexDir string) (*sql.DB, string, error) {
	if err := os.MkdirAll(indexDir, 0o755); err != nil {
		return nil, "", err
	}
	dbPath := filepath.Join(indexDir, "index.sqlite")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, "", err
	}
	if err := migrate(db); err != nil {
		return nil, "", err
	}
	if _, err := db.Exec("PRAGMA journal_mode=WAL;"); err != nil {
		return nil, "", err
	}
	if _, err := db.Exec("PRAGMA foreign_keys=ON;"); err != nil {
		return nil, "", err
	}
	if _, err := db.Exec("PRAGMA synchronous=NORMAL;"); err != nil {
		return nil, "", err
	}
	return db, dbPath, nil
}

func migrate(db *sql.DB) error {
	schema := []string{
		`CREATE TABLE IF NOT EXISTS collections (
			name TEXT PRIMARY KEY,
			path TEXT NOT NULL,
			mask TEXT
		);`,
		`CREATE TABLE IF NOT EXISTS path_contexts (
			virtual_path TEXT PRIMARY KEY,
			description TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS documents (
			docid TEXT PRIMARY KEY,
			collection TEXT NOT NULL,
			relpath TEXT NOT NULL,
			title TEXT,
			content TEXT,
			hash TEXT NOT NULL,
			updated_at INTEGER NOT NULL,
			FOREIGN KEY(collection) REFERENCES collections(name)
		);`,
		`CREATE UNIQUE INDEX IF NOT EXISTS documents_collection_relpath_idx ON documents(collection, relpath);`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS documents_fts USING fts5(
			title,
			content,
			docid UNINDEXED,
			tokenize='porter'
		);`,
		`CREATE TABLE IF NOT EXISTS content_vectors (
			hash TEXT NOT NULL,
			seq INTEGER NOT NULL,
			pos INTEGER NOT NULL,
			vector BLOB NOT NULL,
			chunk_text TEXT,
			PRIMARY KEY (hash, seq)
		);`,
	}
	for i, stmt := range schema {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("migration %d: %w", i, err)
		}
	}
	return nil
}

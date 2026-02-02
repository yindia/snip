package indexer

import (
	"context"
	"database/sql"
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"snip/internal/ignore"
	"snip/internal/util"
)

type Collection struct {
	Name       string
	Path       string
	Extensions string
}

type Stats struct {
	Collections int
	Documents   int
	Updated     int
	Skipped     int
	Removed     int
	Errors      int
}

type job struct {
	AbsPath string
	RelPath string
}

type result struct {
	RelPath string
	Title   string
	Content string
	Hash    string
	DocID   string
	Err     error
}

// Update scans collections and updates the index.
func Update(ctx context.Context, db *sql.DB, collections []Collection) (Stats, error) {
	stats := Stats{Collections: len(collections)}
	for _, col := range collections {
		colStats, err := updateCollection(ctx, db, col)
		if err != nil {
			return stats, err
		}
		stats.Documents += colStats.Documents
		stats.Updated += colStats.Updated
		stats.Skipped += colStats.Skipped
		stats.Removed += colStats.Removed
		stats.Errors += colStats.Errors
	}
	return stats, nil
}

func updateCollection(ctx context.Context, db *sql.DB, col Collection) (Stats, error) {
	stats := Stats{}
	util.Debugf("indexing collection %s (%s)", col.Name, col.Path)
	ignoreMatcher := ignore.NewMatcher(col.Path)
	jobs := make(chan job, 128)
	results := make(chan result, 128)
	var wg sync.WaitGroup
	workerCount := runtime.NumCPU()
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				content, err := os.ReadFile(j.AbsPath)
				if err != nil {
					results <- result{RelPath: j.RelPath, Err: err}
					continue
				}
				hash := util.HashContent(content)
				docid := util.DocIDFromPath(col.Name, j.RelPath)
				title := util.TitleFromMarkdown(string(content), j.RelPath)
				results <- result{RelPath: j.RelPath, Title: title, Content: string(content), Hash: hash, DocID: docid}
			}
		}()
	}

	go func() {
		_ = filepath.WalkDir(col.Path, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				results <- result{RelPath: path, Err: err}
				return nil
			}
			if d.IsDir() {
				name := d.Name()
				if strings.HasPrefix(name, ".") && name != "." {
					if name == ".git" || name == ".snip" {
						return filepath.SkipDir
					}
				}
				if path != col.Path && ignoreMatcher.Ignored(path, true) {
					return filepath.SkipDir
				}
				return nil
			}
			if ignoreMatcher.Ignored(path, false) {
				return nil
			}
			if !matchesExtensions(path, col.Extensions, col.Path) {
				return nil
			}
			rel, err := filepath.Rel(col.Path, path)
			if err != nil {
				return nil
			}
			jobs <- job{AbsPath: path, RelPath: filepath.ToSlash(rel)}
			return nil
		})
		close(jobs)
		wg.Wait()
		close(results)
	}()

	seen := make(map[string]struct{})
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return stats, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	selectStmt, err := tx.PrepareContext(ctx, `SELECT docid, hash FROM documents WHERE collection = ? AND relpath = ?`)
	if err != nil {
		return stats, err
	}
	defer selectStmt.Close()

	deleteDocStmt, err := tx.PrepareContext(ctx, `DELETE FROM documents WHERE docid = ?`)
	if err != nil {
		return stats, err
	}
	defer deleteDocStmt.Close()

	deleteFTSStmt, err := tx.PrepareContext(ctx, `DELETE FROM documents_fts WHERE docid = ?`)
	if err != nil {
		return stats, err
	}
	defer deleteFTSStmt.Close()

	insertDocStmt, err := tx.PrepareContext(ctx, `INSERT INTO documents(docid, collection, relpath, title, content, hash, updated_at) VALUES(?,?,?,?,?,?,?)`)
	if err != nil {
		return stats, err
	}
	defer insertDocStmt.Close()

	insertFTSStmt, err := tx.PrepareContext(ctx, `INSERT INTO documents_fts(title, content, docid) VALUES(?,?,?)`)
	if err != nil {
		return stats, err
	}
	defer insertFTSStmt.Close()

	for res := range results {
		if res.Err != nil {
			stats.Errors++
			continue
		}
		seen[res.RelPath] = struct{}{}
		stats.Documents++

		var existingDocID string
		var existingHash string
		err = selectStmt.QueryRowContext(ctx, col.Name, res.RelPath).Scan(&existingDocID, &existingHash)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return stats, err
		}
		if err == nil && existingHash == res.Hash {
			stats.Skipped++
			continue
		}
		if err == nil {
			if _, err = deleteDocStmt.ExecContext(ctx, existingDocID); err != nil {
				return stats, err
			}
			if _, err = deleteFTSStmt.ExecContext(ctx, existingDocID); err != nil {
				return stats, err
			}
			if _, err = tx.ExecContext(ctx, `DELETE FROM content_vectors WHERE hash = ?`, existingHash); err != nil {
				return stats, err
			}
		}
		updatedAt := time.Now().Unix()
		if _, err = insertDocStmt.ExecContext(ctx, res.DocID, col.Name, res.RelPath, res.Title, res.Content, res.Hash, updatedAt); err != nil {
			return stats, err
		}
		if _, err = insertFTSStmt.ExecContext(ctx, res.Title, res.Content, res.DocID); err != nil {
			return stats, err
		}
		stats.Updated++
	}

	rows, err := tx.QueryContext(ctx, `SELECT docid, relpath, hash FROM documents WHERE collection = ?`, col.Name)
	if err != nil {
		return stats, err
	}
	defer rows.Close()
	for rows.Next() {
		var docid, relpath, hash string
		if err := rows.Scan(&docid, &relpath, &hash); err != nil {
			return stats, err
		}
		if _, ok := seen[relpath]; ok {
			continue
		}
		if _, err := deleteDocStmt.ExecContext(ctx, docid); err != nil {
			return stats, err
		}
		if _, err := deleteFTSStmt.ExecContext(ctx, docid); err != nil {
			return stats, err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM content_vectors WHERE hash = ?`, hash); err != nil {
			return stats, err
		}
		stats.Removed++
	}
	if err := rows.Err(); err != nil {
		return stats, err
	}

	if err := tx.Commit(); err != nil {
		return stats, err
	}
	return stats, nil
}

func matchesExtensions(fullPath string, extensions string, base string) bool {
	rel, err := filepath.Rel(base, fullPath)
	if err != nil {
		return false
	}
	rel = filepath.ToSlash(rel)
	if extensions != "" {
		ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(fullPath)), ".")
		for _, part := range strings.Split(extensions, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			if strings.ContainsAny(part, "*?[]/") {
				part = filepath.ToSlash(part)
				target := rel
				if !strings.Contains(part, "/") {
					target = path.Base(rel)
				}
				ok, err := path.Match(part, target)
				if err == nil && ok {
					return true
				}
				continue
			}
			part = strings.TrimPrefix(part, ".")
			part = strings.ToLower(part)
			if part != "" && part == ext {
				return true
			}
		}
		return false
	}
	lower := strings.ToLower(fullPath)
	return strings.HasSuffix(lower, ".md") || strings.HasSuffix(lower, ".markdown") || strings.HasSuffix(lower, ".mdx") || strings.HasSuffix(lower, ".mdown")
}

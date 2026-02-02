package embed

import (
	"context"
	"database/sql"
	"path/filepath"
	"runtime"
	"sync"

	"snip/internal/ignore"
)

type EmbedStats struct {
	Documents int
	Embedded  int
	Skipped   int
	Errors    int
}

type docJob struct {
	Hash    string
	Title   string
	Content string
}

type docResult struct {
	Hash    string
	Chunks  []Chunk
	Vectors [][]float32
	Err     error
}

// EmbedDocuments generates embeddings for documents and stores them.
func EmbedDocuments(ctx context.Context, db *sql.DB, embedder Embedder, force bool) (EmbedStats, error) {
	stats := EmbedStats{}
	rows, err := db.QueryContext(ctx, `
		SELECT d.hash, d.title, d.content, d.collection, d.relpath, c.path
		FROM documents d
		JOIN collections c ON c.name = d.collection`)
	if err != nil {
		return stats, err
	}
	defer rows.Close()

	jobs := make(chan docJob, 64)
	results := make(chan docResult, 64)
	var wg sync.WaitGroup
	workerCount := runtime.NumCPU()
	var mu sync.Mutex
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				chunks := ChunkText(job.Content, 800, 120)
				if len(chunks) == 0 {
					results <- docResult{Hash: job.Hash}
					continue
				}
				texts := make([]string, 0, len(chunks))
				formatter, _ := embedder.(TextFormatter)
				for _, c := range chunks {
					text := c.Text
					if formatter != nil {
						text = formatter.FormatDocument(job.Title, text)
					}
					texts = append(texts, text)
				}
				vecs, err := embedder.Embed(texts)
				results <- docResult{Hash: job.Hash, Chunks: chunks, Vectors: vecs, Err: err}
			}
		}()
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	producerErr := make(chan error, 1)
	go func() {
		defer close(jobs)
		var scanErr error
		ignoreMatchers := make(map[string]*ignore.Matcher)
		for rows.Next() {
			var hash, title, content, collection, relpath, basePath string
			if err := rows.Scan(&hash, &title, &content, &collection, &relpath, &basePath); err != nil {
				scanErr = err
				break
			}
			mu.Lock()
			stats.Documents++
			mu.Unlock()
			matcher := ignoreMatchers[collection]
			if matcher == nil {
				matcher = ignore.NewMatcher(basePath)
				ignoreMatchers[collection] = matcher
			}
			absPath := filepath.Join(basePath, filepath.FromSlash(relpath))
			if matcher.Ignored(absPath, false) {
				mu.Lock()
				stats.Skipped++
				mu.Unlock()
				continue
			}
			if !force {
				var exists int
				err := db.QueryRowContext(ctx, `SELECT 1 FROM content_vectors WHERE hash = ? LIMIT 1`, hash).Scan(&exists)
				if err == nil {
					mu.Lock()
					stats.Skipped++
					mu.Unlock()
					continue
				}
			}
			jobs <- docJob{Hash: hash, Title: title, Content: content}
		}
		if scanErr != nil {
			producerErr <- scanErr
			return
		}
		if err := rows.Err(); err != nil {
			producerErr <- err
			return
		}
		producerErr <- nil
	}()

	for res := range results {
		if res.Err != nil {
			mu.Lock()
			stats.Errors++
			mu.Unlock()
			continue
		}
		if len(res.Chunks) == 0 {
			mu.Lock()
			stats.Skipped++
			mu.Unlock()
			continue
		}
		if len(res.Vectors) != len(res.Chunks) {
			mu.Lock()
			stats.Errors++
			mu.Unlock()
			continue
		}
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return stats, err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM content_vectors WHERE hash = ?`, res.Hash); err != nil {
			_ = tx.Rollback()
			return stats, err
		}
		for i, chunk := range res.Chunks {
			if i >= len(res.Vectors) {
				continue
			}
			vecBytes := EncodeVector(res.Vectors[i])
			if _, err := tx.ExecContext(ctx, `INSERT INTO content_vectors(hash, seq, pos, vector, chunk_text) VALUES(?,?,?,?,?)`, res.Hash, chunk.Seq, chunk.Pos, vecBytes, chunk.Text); err != nil {
				_ = tx.Rollback()
				return stats, err
			}
		}
		if err := tx.Commit(); err != nil {
			return stats, err
		}
		mu.Lock()
		stats.Embedded++
		mu.Unlock()
	}
	if err := <-producerErr; err != nil {
		return stats, err
	}
	return stats, nil
}

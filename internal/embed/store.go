package embed

import (
	"context"
	"database/sql"
	"runtime"
	"sync"
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
	rows, err := db.QueryContext(ctx, `SELECT hash, title, content FROM documents`)
	if err != nil {
		return stats, err
	}
	defer rows.Close()

	jobs := make(chan docJob, 64)
	results := make(chan docResult, 64)
	var wg sync.WaitGroup
	workerCount := runtime.NumCPU()
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

	for rows.Next() {
		var hash, title, content string
		if err := rows.Scan(&hash, &title, &content); err != nil {
			return stats, err
		}
		stats.Documents++
		if !force {
			var exists int
			err := db.QueryRowContext(ctx, `SELECT 1 FROM content_vectors WHERE hash = ? LIMIT 1`, hash).Scan(&exists)
			if err == nil {
				stats.Skipped++
				continue
			}
		}
		jobs <- docJob{Hash: hash, Title: title, Content: content}
	}
	close(jobs)
	if err := rows.Err(); err != nil {
		return stats, err
	}

	for res := range results {
		if res.Err != nil {
			stats.Errors++
			continue
		}
		if len(res.Chunks) == 0 {
			stats.Skipped++
			continue
		}
		if len(res.Vectors) != len(res.Chunks) {
			stats.Errors++
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
		stats.Embedded++
	}
	return stats, nil
}

package search

import (
	"context"
	"database/sql"
	"math"
	"regexp"
	"sort"
	"strings"
	"sync"

	"snip/internal/embed"
	"snip/internal/rerank"
)

type rrfList struct {
	items  []Result
	weight float64
}

var ftsTokenRe = regexp.MustCompile(`[A-Za-z0-9_]+`)

// FTSSearch runs a BM25 search using FTS5.
func FTSSearch(ctx context.Context, db *sql.DB, query string, opts Options) ([]Result, error) {
	if strings.TrimSpace(query) == "" {
		return nil, nil
	}
	limit := opts.Limit
	if limit <= 0 {
		limit = 5
	}
	collection := opts.Collection
	if opts.All {
		collection = ""
	}
	rows, err := queryFTS(ctx, db, query, collection, limit)
	if err != nil && isFTSSyntaxError(err) {
		clean := sanitizeFTSQuery(query)
		if clean == "" {
			return nil, nil
		}
		if clean != query {
			rows, err = queryFTS(ctx, db, clean, collection, limit)
		}
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	results := []Result{}
	for rows.Next() {
		var r Result
		if err := rows.Scan(&r.DocID, &r.Collection, &r.RelPath, &r.Title, &r.Content, &r.Score); err != nil {
			return nil, err
		}
		r.Snippet = buildSnippet(r.Content, query)
		if !opts.Full {
			r.Content = ""
		}
		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	normalizeScores(results)
	results = applyMinScore(results, opts.MinScore)
	attachContexts(ctx, db, results)
	return results, nil
}

func queryFTS(ctx context.Context, db *sql.DB, query string, collection string, limit int) (*sql.Rows, error) {
	return db.QueryContext(ctx, `
		SELECT d.docid, d.collection, d.relpath, d.title, d.content, -bm25(documents_fts) AS score
		FROM documents_fts
		JOIN documents d ON d.docid = documents_fts.docid
		WHERE documents_fts MATCH ?
		AND (? = '' OR d.collection = ?)
		ORDER BY score DESC
		LIMIT ?`, query, collection, collection, limit)
}

// VectorSearch runs a vector similarity search against stored vectors.
func VectorSearch(ctx context.Context, db *sql.DB, embedder embed.Embedder, query string, opts Options) ([]Result, error) {
	if strings.TrimSpace(query) == "" {
		return nil, nil
	}
	limit := opts.Limit
	if limit <= 0 {
		limit = 5
	}
	collection := opts.Collection
	if opts.All {
		collection = ""
	}
	queryText := query
	if formatter, ok := embedder.(embed.TextFormatter); ok {
		queryText = formatter.FormatQuery(query)
	}
	queryVecs, err := embedder.Embed([]string{queryText})
	if err != nil {
		return nil, err
	}
	if len(queryVecs) == 0 {
		return nil, nil
	}
	qvec := queryVecs[0]
	rows, err := db.QueryContext(ctx, `
		SELECT d.docid, d.collection, d.relpath, d.title, d.content, v.vector
		FROM content_vectors v
		JOIN documents d ON d.hash = v.hash
		WHERE (? = '' OR d.collection = ?)`, collection, collection)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type agg struct {
		res   Result
		score float64
	}
	byDoc := map[string]*agg{}
	for rows.Next() {
		var r Result
		var vecBytes []byte
		if err := rows.Scan(&r.DocID, &r.Collection, &r.RelPath, &r.Title, &r.Content, &vecBytes); err != nil {
			return nil, err
		}
		vec := embed.DecodeVector(vecBytes)
		if vec == nil {
			continue
		}
		sim := cosine(qvec, vec)
		ag, ok := byDoc[r.DocID]
		if !ok || sim > ag.score {
			copyRes := r
			byDoc[r.DocID] = &agg{res: copyRes, score: sim}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	results := make([]Result, 0, len(byDoc))
	for _, ag := range byDoc {
		ag.res.Score = ag.score
		ag.res.Snippet = buildSnippet(ag.res.Content, query)
		if !opts.Full {
			ag.res.Content = ""
		}
		results = append(results, ag.res)
	}
	sort.Slice(results, func(i, j int) bool { return results[i].Score > results[j].Score })
	normalizeScores(results)
	results = applyMinScore(results, opts.MinScore)
	if len(results) > limit {
		results = results[:limit]
	}
	attachContexts(ctx, db, results)
	return results, nil
}

// HybridSearch combines FTS and vector retrieval using RRF and reranking.
func HybridSearch(ctx context.Context, db *sql.DB, embedder embed.Embedder, rr rerank.Reranker, expander Expander, query string, opts Options) ([]Result, error) {
	if strings.TrimSpace(query) == "" {
		return nil, nil
	}
	queries := expandQueriesWith(expander, query)
	lists := make([]rrfList, 0, len(queries)*2)
	var mu sync.Mutex
	var wg sync.WaitGroup
	var firstErr error
	for i, q := range queries {
		w := 1.0
		if i == 0 {
			w = 2.0
		}
		wg.Add(2)
		go func(q string, weight float64) {
			defer wg.Done()
			res, err := FTSSearch(ctx, db, q, Options{Limit: 50, Collection: opts.Collection, All: opts.All, MinScore: opts.MinScore, Full: true})
			if err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
				return
			}
			mu.Lock()
			lists = append(lists, rrfList{items: res, weight: weight})
			mu.Unlock()
		}(q, w)
		go func(q string, weight float64) {
			defer wg.Done()
			res, err := VectorSearch(ctx, db, embedder, q, Options{Limit: 50, Collection: opts.Collection, All: opts.All, MinScore: opts.MinScore, Full: true})
			if err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
				return
			}
			mu.Lock()
			lists = append(lists, rrfList{items: res, weight: weight})
			mu.Unlock()
		}(q, w)
	}
	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}

	fused := rrfFuse(lists)
	if len(fused) == 0 {
		return nil, nil
	}
	if len(fused) > 30 {
		fused = fused[:30]
	}

	normalizeScores(fused)

	rerankDocs := make([]rerank.Doc, 0, len(fused))
	for _, res := range fused {
		rerankDocs = append(rerankDocs, rerank.Doc{Title: res.Title, Content: res.Content, Context: res.Context})
	}
	rerankScores, err := rr.Rerank(query, rerankDocs)
	if err != nil {
		return nil, err
	}
	rerankScores = normalizeFloatScores(rerankScores)
	for i := range fused {
		weight := blendWeight(i)
		fused[i].Score = weight*fused[i].Score + (1.0-weight)*rerankScores[i]
		fused[i].Snippet = buildSnippet(fused[i].Content, query)
		if !opts.Full {
			fused[i].Content = ""
		}
	}
	sort.Slice(fused, func(i, j int) bool { return fused[i].Score > fused[j].Score })
	if opts.Limit > 0 && len(fused) > opts.Limit {
		fused = fused[:opts.Limit]
	}
	attachContexts(ctx, db, fused)
	return fused, nil
}

func blendWeight(rank int) float64 {
	pos := rank + 1
	if pos <= 3 {
		return 0.75
	}
	if pos <= 10 {
		return 0.60
	}
	return 0.40
}

func cosine(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var sum float64
	for i := range a {
		sum += float64(a[i] * b[i])
	}
	if math.IsNaN(sum) || math.IsInf(sum, 0) {
		return 0
	}
	return sum
}

func buildSnippet(content, query string) string {
	if content == "" {
		return ""
	}
	lower := strings.ToLower(content)
	q := strings.ToLower(query)
	idx := strings.Index(lower, q)
	if idx == -1 {
		return truncate(content, 200)
	}
	start := idx - 80
	if start < 0 {
		start = 0
	}
	end := idx + len(q) + 120
	if end > len(content) {
		end = len(content)
	}
	snippet := content[start:end]
	return strings.TrimSpace(snippet)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return strings.TrimSpace(s[:max]) + "..."
}

func sanitizeFTSQuery(query string) string {
	tokens := ftsTokenRe.FindAllString(query, -1)
	return strings.Join(tokens, " ")
}

func isFTSSyntaxError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "fts5: syntax error")
}

func expandQueries(query string) []string {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil
	}
	alts := []string{query}
	lower := strings.ToLower(query)
	if lower != query {
		alts = append(alts, lower)
	}
	clean := strings.NewReplacer("-", " ", "_", " ", ":", " ", "/", " ").Replace(lower)
	clean = strings.Join(strings.Fields(clean), " ")
	if clean != "" && clean != lower {
		alts = append(alts, clean)
	}
	stop := map[string]struct{}{"the": {}, "a": {}, "an": {}, "and": {}, "or": {}, "of": {}, "for": {}, "to": {}, "in": {}}
	parts := strings.Fields(clean)
	filtered := make([]string, 0, len(parts))
	for _, p := range parts {
		if _, ok := stop[p]; ok {
			continue
		}
		filtered = append(filtered, p)
	}
	alt2 := strings.Join(filtered, " ")
	if alt2 != "" && alt2 != clean {
		alts = append(alts, alt2)
	}
	return unique(alts, 3)
}

func expandQueriesWith(expander Expander, query string) []string {
	base := expandQueries(query)
	if expander == nil {
		return base
	}
	alts, err := expander.Expand(query)
	if err != nil || len(alts) == 0 {
		return base
	}
	out := []string{query}
	for _, alt := range alts {
		if strings.TrimSpace(alt) == "" || strings.EqualFold(alt, query) {
			continue
		}
		out = append(out, alt)
		if len(out) >= 3 {
			break
		}
	}
	if len(out) == 1 {
		return base
	}
	return out
}

func unique(items []string, max int) []string {
	seen := map[string]struct{}{}
	out := []string{}
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
		if len(out) >= max {
			break
		}
	}
	return out
}

func attachContexts(ctx context.Context, db *sql.DB, results []Result) {
	if len(results) == 0 {
		return
	}
	ctxMap, err := loadContexts(ctx, db)
	if err != nil {
		return
	}
	for i := range results {
		results[i].Context = findContext(ctxMap, results[i].Collection, results[i].RelPath)
	}
}

func loadContexts(ctx context.Context, db *sql.DB) (map[string]string, error) {
	rows, err := db.QueryContext(ctx, `SELECT virtual_path, description FROM path_contexts`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var path, desc string
		if err := rows.Scan(&path, &desc); err != nil {
			return nil, err
		}
		out[path] = desc
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func findContext(ctxMap map[string]string, collection, relpath string) string {
	if len(ctxMap) == 0 {
		return ""
	}
	full := "snip://" + collection + "/" + relpath
	if desc, ok := ctxMap[full]; ok {
		return desc
	}
	root := "snip://" + collection
	if desc, ok := ctxMap[root]; ok {
		return desc
	}
	best := ""
	bestLen := 0
	prefix := "snip://" + collection + "/"
	for k, v := range ctxMap {
		if !strings.HasPrefix(k, prefix) {
			continue
		}
		if strings.HasPrefix(full, k) && len(k) > bestLen {
			best = v
			bestLen = len(k)
		}
	}
	return best
}

func rrfFuse(lists []rrfList) []Result {
	scores := map[string]float64{}
	best := map[string]Result{}
	for _, list := range lists {
		for i, res := range list.items {
			rank := i + 1
			score := list.weight * (1.0 / (60.0 + float64(rank)))
			if rank == 1 {
				score += 0.05
			} else if rank <= 3 {
				score += 0.02
			}
			scores[res.DocID] += score
			if _, ok := best[res.DocID]; !ok {
				best[res.DocID] = res
			}
		}
	}
	out := make([]Result, 0, len(scores))
	for docid, score := range scores {
		res := best[docid]
		res.Score = score
		out = append(out, res)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	return out
}

func normalizeScores(results []Result) {
	if len(results) == 0 {
		return
	}
	min := results[0].Score
	max := results[0].Score
	for _, r := range results[1:] {
		if r.Score < min {
			min = r.Score
		}
		if r.Score > max {
			max = r.Score
		}
	}
	denom := max - min
	if denom <= 0 {
		for i := range results {
			results[i].Score = 1.0
		}
		return
	}
	for i := range results {
		results[i].Score = (results[i].Score - min) / denom
	}
}

func applyMinScore(results []Result, min float64) []Result {
	if min <= 0 {
		return results
	}
	out := make([]Result, 0, len(results))
	for _, r := range results {
		if r.Score >= min {
			out = append(out, r)
		}
	}
	return out
}

func normalizeFloatScores(scores []float64) []float64 {
	if len(scores) == 0 {
		return scores
	}
	min := scores[0]
	max := scores[0]
	for _, v := range scores[1:] {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	denom := max - min
	out := make([]float64, len(scores))
	if denom <= 0 {
		for i := range scores {
			out[i] = 1.0
		}
		return out
	}
	for i, v := range scores {
		out[i] = (v - min) / denom
	}
	return out
}

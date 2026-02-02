package mcpserver

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"snip/internal/embed"
	"snip/internal/rerank"
	"snip/internal/search"
	"snip/internal/util"
)

const (
	defaultLimit    = 10
	maxLimit        = 100
	defaultMaxBytes = 10240
)

type Adapter struct {
	db       *sql.DB
	embedder embed.Embedder
	reranker rerank.Reranker
	expander search.Expander
}

func NewAdapter(db *sql.DB, embedder embed.Embedder, rr rerank.Reranker, expander search.Expander) *Adapter {
	return &Adapter{
		db:       db,
		embedder: embedder,
		reranker: rr,
		expander: expander,
	}
}

type SearchInput struct {
	Query      string  `json:"query" jsonschema:"search query"`
	Limit      int     `json:"limit,omitempty" jsonschema:"max results (default 10)"`
	MinScore   float64 `json:"minScore,omitempty" jsonschema:"minimum score threshold"`
	Collection string  `json:"collection,omitempty" jsonschema:"collection name"`
}

type SearchResult struct {
	DocID   string  `json:"docid"`
	File    string  `json:"file"`
	Title   string  `json:"title"`
	Score   float64 `json:"score"`
	Context *string `json:"context"`
	Snippet string  `json:"snippet"`
}

type SearchOutput struct {
	Results []SearchResult `json:"results"`
}

type GetInput struct {
	File        string `json:"file" jsonschema:"path, docid (#abc123def4567890), or path:line"`
	FromLine    int    `json:"fromLine,omitempty" jsonschema:"starting line number"`
	MaxLines    int    `json:"maxLines,omitempty" jsonschema:"maximum lines to return"`
	LineNumbers bool   `json:"lineNumbers,omitempty" jsonschema:"include line numbers"`
}

type GetOutput struct {
	DocID       string  `json:"docid"`
	File        string  `json:"file"`
	Title       string  `json:"title"`
	Content     string  `json:"content"`
	Context     *string `json:"context"`
	LineNumbers bool    `json:"lineNumbers"`
	FromLine    int     `json:"fromLine,omitempty"`
	MaxLines    int     `json:"maxLines,omitempty"`
	Truncated   bool    `json:"truncated,omitempty"`
}

type MultiGetInput struct {
	Pattern     string `json:"pattern" jsonschema:"glob pattern or comma-separated list"`
	MaxLines    int    `json:"maxLines,omitempty" jsonschema:"maximum lines per document"`
	MaxBytes    int    `json:"maxBytes,omitempty" jsonschema:"maximum bytes per document (default 10240)"`
	LineNumbers bool   `json:"lineNumbers,omitempty" jsonschema:"include line numbers"`
}

type SkippedItem struct {
	Item   string `json:"item"`
	Reason string `json:"reason"`
}

type MultiGetOutput struct {
	Results []GetOutput   `json:"results"`
	Skipped []SkippedItem `json:"skipped,omitempty"`
}

type StatusOutput struct {
	TotalDocuments int                `json:"totalDocuments"`
	NeedsEmbedding int                `json:"needsEmbedding"`
	HasVectorIndex bool               `json:"hasVectorIndex"`
	Collections    []CollectionStatus `json:"collections"`
}

type CollectionStatus struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Extensions  string `json:"extensions" jsonschema:"comma-separated file extensions"`
	Documents   int    `json:"documents"`
	LastUpdated string `json:"lastUpdated"`
}

func (a *Adapter) Search(ctx context.Context, _ *mcp.CallToolRequest, input SearchInput) (*mcp.CallToolResult, SearchOutput, error) {
	query := strings.TrimSpace(input.Query)
	if query == "" {
		return summaryResult("no query provided"), SearchOutput{Results: nil}, nil
	}
	limit := clampLimit(input.Limit)
	opts := search.Options{
		Limit:      limit,
		Collection: input.Collection,
		MinScore:   input.MinScore,
		Full:       true,
	}
	results, err := search.FTSSearch(ctx, a.db, query, opts)
	if err != nil {
		return nil, SearchOutput{}, err
	}
	out := SearchOutput{Results: formatSearchResults(results, query)}
	return summaryResult(fmt.Sprintf("snip_search: %d results", len(out.Results))), out, nil
}

func (a *Adapter) VSearch(ctx context.Context, _ *mcp.CallToolRequest, input SearchInput) (*mcp.CallToolResult, SearchOutput, error) {
	query := strings.TrimSpace(input.Query)
	if query == "" {
		return summaryResult("no query provided"), SearchOutput{Results: nil}, nil
	}
	available, reason, err := a.vectorAvailable(ctx)
	if err != nil {
		return nil, SearchOutput{}, err
	}
	if !available {
		return nil, SearchOutput{}, errors.New(reason)
	}
	limit := clampLimit(input.Limit)
	opts := search.Options{
		Limit:      limit,
		Collection: input.Collection,
		MinScore:   input.MinScore,
		Full:       true,
	}
	results, err := search.VectorSearch(ctx, a.db, a.embedder, query, opts)
	if err != nil {
		return nil, SearchOutput{}, err
	}
	out := SearchOutput{Results: formatSearchResults(results, query)}
	return summaryResult(fmt.Sprintf("snip_vsearch: %d results", len(out.Results))), out, nil
}

func (a *Adapter) Query(ctx context.Context, _ *mcp.CallToolRequest, input SearchInput) (*mcp.CallToolResult, SearchOutput, error) {
	query := strings.TrimSpace(input.Query)
	if query == "" {
		return summaryResult("no query provided"), SearchOutput{Results: nil}, nil
	}
	limit := clampLimit(input.Limit)
	opts := search.Options{
		Limit:      limit,
		Collection: input.Collection,
		MinScore:   input.MinScore,
		Full:       true,
	}
	available, _, err := a.vectorAvailable(ctx)
	if err != nil {
		return nil, SearchOutput{}, err
	}
	var results []search.Result
	if available {
		results, err = search.HybridSearch(ctx, a.db, a.embedder, a.reranker, a.expander, query, opts)
	} else {
		results, err = search.FTSSearch(ctx, a.db, query, opts)
	}
	if err != nil {
		return nil, SearchOutput{}, err
	}
	out := SearchOutput{Results: formatSearchResults(results, query)}
	prefix := "snip_query"
	if !available {
		prefix = "snip_query (keyword fallback)"
	}
	return summaryResult(fmt.Sprintf("%s: %d results", prefix, len(out.Results))), out, nil
}

func (a *Adapter) Get(ctx context.Context, _ *mcp.CallToolRequest, input GetInput) (*mcp.CallToolResult, GetOutput, error) {
	docid, path, line := parseDocSpecifier(input.File)
	if input.FromLine > 0 {
		line = input.FromLine
	}
	if docid == "" && strings.TrimSpace(path) == "" {
		return nil, GetOutput{}, errors.New("file is required")
	}
	doc, err := a.fetchDocument(ctx, docid, path)
	if err != nil {
		return nil, GetOutput{}, err
	}
	content, fromLine, maxLines, err := sliceContent(doc.Content, line, input.MaxLines)
	if err != nil {
		return nil, GetOutput{}, err
	}
	if input.LineNumbers {
		content = addLineNumbers(content, fromLine)
	}
	ctxText := a.lookupContext(ctx, doc.Collection, doc.RelPath)
	out := GetOutput{
		DocID:       "#" + doc.DocID,
		File:        doc.Collection + "/" + doc.RelPath,
		Title:       doc.Title,
		Content:     content,
		Context:     toContextPtr(ctxText),
		LineNumbers: input.LineNumbers,
	}
	if fromLine > 1 || input.MaxLines > 0 {
		out.FromLine = fromLine
		out.MaxLines = maxLines
	}
	return summaryResult(fmt.Sprintf("snip_get: %s", out.File)), out, nil
}

func (a *Adapter) MultiGet(ctx context.Context, _ *mcp.CallToolRequest, input MultiGetInput) (*mcp.CallToolResult, MultiGetOutput, error) {
	pattern := strings.TrimSpace(input.Pattern)
	if pattern == "" {
		return nil, MultiGetOutput{}, errors.New("pattern is required")
	}
	maxBytes := input.MaxBytes
	if maxBytes <= 0 {
		maxBytes = defaultMaxBytes
	}

	var docs []docRecord
	var skipped []SkippedItem
	if strings.ContainsAny(pattern, "*?[]") {
		docs = a.fetchByGlob(ctx, pattern)
	} else {
		items := splitList(pattern)
		for _, item := range items {
			docid, path, line := parseDocSpecifier(item)
			doc, err := a.fetchDocument(ctx, docid, path)
			if err != nil {
				skipped = append(skipped, SkippedItem{Item: item, Reason: err.Error()})
				continue
			}
			doc.Line = line
			docs = append(docs, *doc)
		}
	}

	results := make([]GetOutput, 0, len(docs))
	for _, doc := range docs {
		content, fromLine, maxLines, err := sliceContent(doc.Content, doc.Line, input.MaxLines)
		if err != nil {
			skipped = append(skipped, SkippedItem{Item: doc.Collection + "/" + doc.RelPath, Reason: err.Error()})
			continue
		}
		if input.LineNumbers {
			content = addLineNumbers(content, fromLine)
		}
		content, truncated := enforceMaxBytes(content, maxBytes)
		ctxText := a.lookupContext(ctx, doc.Collection, doc.RelPath)
		out := GetOutput{
			DocID:       "#" + doc.DocID,
			File:        doc.Collection + "/" + doc.RelPath,
			Title:       doc.Title,
			Content:     content,
			Context:     toContextPtr(ctxText),
			LineNumbers: input.LineNumbers,
			Truncated:   truncated,
		}
		if fromLine > 1 || input.MaxLines > 0 {
			out.FromLine = fromLine
			out.MaxLines = maxLines
		}
		results = append(results, out)
	}

	out := MultiGetOutput{Results: results}
	if len(skipped) > 0 {
		out.Skipped = skipped
	}
	return summaryResult(fmt.Sprintf("snip_multi_get: %d documents", len(results))), out, nil
}

func (a *Adapter) Status(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, StatusOutput, error) {
	var total int
	if err := a.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM documents`).Scan(&total); err != nil {
		return nil, StatusOutput{}, err
	}
	var needsEmbedding int
	if err := a.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM documents d
		WHERE NOT EXISTS (SELECT 1 FROM content_vectors v WHERE v.hash = d.hash)
	`).Scan(&needsEmbedding); err != nil {
		return nil, StatusOutput{}, err
	}
	var vectorCount int
	if err := a.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM content_vectors`).Scan(&vectorCount); err != nil {
		return nil, StatusOutput{}, err
	}
	collections, err := a.collectionStatus(ctx)
	if err != nil {
		return nil, StatusOutput{}, err
	}
	out := StatusOutput{
		TotalDocuments: total,
		NeedsEmbedding: needsEmbedding,
		HasVectorIndex: vectorCount > 0,
		Collections:    collections,
	}
	return summaryResult(fmt.Sprintf("snip_status: %d documents", total)), out, nil
}

func (a *Adapter) ReadResource(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	rawURI := req.Params.URI
	if !strings.HasPrefix(rawURI, "snip://") {
		return nil, mcp.ResourceNotFoundError(rawURI)
	}
	path := strings.TrimPrefix(rawURI, "snip://")
	path = strings.TrimPrefix(path, "/")
	decoded, err := url.PathUnescape(path)
	if err == nil {
		path = decoded
	}

	doc, err := a.getDocByPath(ctx, path)
	if err != nil {
		doc, err = a.getDocBySuffix(ctx, path)
	}
	if err != nil {
		return nil, mcp.ResourceNotFoundError(rawURI)
	}
	ctxText := a.lookupContext(ctx, doc.Collection, doc.RelPath)
	content := doc.Content
	if ctxText != "" {
		content = fmt.Sprintf("Context: %s\n\n%s", ctxText, content)
	}
	content = addLineNumbers(content, 1)
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{{
			URI:      rawURI,
			MIMEType: "text/markdown",
			Text:     content,
		}},
	}, nil
}

func (a *Adapter) QueryPrompt(_ context.Context, _ *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	text := strings.TrimSpace(`
Use snip_search for fast keyword/BM25 lookups (exact terms, symbols, filenames).
Use snip_vsearch for semantic similarity when embeddings are available.
Use snip_query for the hybrid pipeline (best overall quality).
Use snip_get to fetch a single document by path or docid (#abc123def4567890).
Use snip_multi_get for glob patterns or comma-separated lists, and cap output with maxBytes.
Resources are available as snip://collection/relative/path and include line numbers by default.
`)
	return &mcp.GetPromptResult{
		Description: "Guidance for SNIP search tools and retrieval",
		Messages: []*mcp.PromptMessage{{
			Role:    "user",
			Content: &mcp.TextContent{Text: text},
		}},
	}, nil
}

func searchInputSchema(defaultMinScore float64) *jsonschema.Schema {
	return &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"query": {
				Type:        "string",
				Description: "search query",
			},
			"limit": {
				Type:        "integer",
				Description: "max results (default 10, max 100)",
				Default:     json.RawMessage(fmt.Sprintf("%d", defaultLimit)),
				Minimum:     jsonschema.Ptr(1.0),
				Maximum:     jsonschema.Ptr(float64(maxLimit)),
			},
			"minScore": {
				Type:        "number",
				Description: "minimum score threshold",
				Default:     json.RawMessage(fmt.Sprintf("%g", defaultMinScore)),
			},
			"collection": {
				Type:        "string",
				Description: "collection name",
			},
		},
		Required: []string{"query"},
	}
}

func getInputSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"file": {
				Type:        "string",
				Description: "path, docid (#abc123def4567890), or path:line",
			},
			"fromLine": {
				Type:        "integer",
				Description: "starting line number",
			},
			"maxLines": {
				Type:        "integer",
				Description: "maximum lines to return",
			},
			"lineNumbers": {
				Type:        "boolean",
				Description: "include line numbers",
				Default:     json.RawMessage("false"),
			},
		},
		Required: []string{"file"},
	}
}

func multiGetInputSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"pattern": {
				Type:        "string",
				Description: "glob pattern or comma-separated list",
			},
			"maxLines": {
				Type:        "integer",
				Description: "maximum lines per document",
			},
			"maxBytes": {
				Type:        "integer",
				Description: "maximum bytes per document (default 10240)",
				Default:     json.RawMessage(fmt.Sprintf("%d", defaultMaxBytes)),
			},
			"lineNumbers": {
				Type:        "boolean",
				Description: "include line numbers",
				Default:     json.RawMessage("false"),
			},
		},
		Required: []string{"pattern"},
	}
}

func clampLimit(limit int) int {
	if limit <= 0 {
		return defaultLimit
	}
	if limit > maxLimit {
		return maxLimit
	}
	return limit
}

func clampScore(score float64) float64 {
	if score < 0 {
		return 0
	}
	if score > 1 {
		return 1
	}
	return score
}

func summaryResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}
}

func formatSearchResults(results []search.Result, query string) []SearchResult {
	out := make([]SearchResult, 0, len(results))
	for _, r := range results {
		file := r.Collection + "/" + r.RelPath
		out = append(out, SearchResult{
			DocID:   "#" + r.DocID,
			File:    file,
			Title:   r.Title,
			Score:   clampScore(r.Score),
			Context: toContextPtr(r.Context),
			Snippet: snippetWithLines(r.Content, query),
		})
	}
	return out
}

func snippetWithLines(content, query string) string {
	if content == "" {
		return ""
	}
	lines := strings.Split(content, "\n")
	if query == "" {
		return formatLines(lines, 1, 5)
	}
	lower := strings.ToLower(content)
	q := strings.ToLower(query)
	idx := strings.Index(lower, q)
	line := 1
	if idx > -1 {
		line = 1 + strings.Count(lower[:idx], "\n")
	}
	start := line - 2
	if start < 1 {
		start = 1
	}
	end := line + 2
	if end > len(lines) {
		end = len(lines)
	}
	return formatLines(lines[start-1:end], start, 0)
}

func formatLines(lines []string, startLine int, maxLines int) string {
	if maxLines > 0 && len(lines) > maxLines {
		lines = lines[:maxLines]
	}
	for i, line := range lines {
		lines[i] = fmt.Sprintf("%4d | %s", startLine+i, line)
	}
	return strings.Join(lines, "\n")
}

func addLineNumbers(content string, startLine int) string {
	lines := strings.Split(content, "\n")
	return formatLines(lines, startLine, 0)
}

func sliceContent(content string, fromLine int, maxLines int) (string, int, int, error) {
	lines := strings.Split(content, "\n")
	if fromLine <= 0 {
		fromLine = 1
	}
	if fromLine > len(lines) {
		return "", fromLine, maxLines, fmt.Errorf("line %d out of range", fromLine)
	}
	end := len(lines)
	if maxLines > 0 && fromLine+maxLines-1 < end {
		end = fromLine + maxLines - 1
	}
	segment := strings.Join(lines[fromLine-1:end], "\n")
	return segment, fromLine, maxLines, nil
}

func enforceMaxBytes(content string, maxBytes int) (string, bool) {
	if maxBytes <= 0 {
		return content, false
	}
	if len(content) <= maxBytes {
		return content, false
	}
	truncated := content[:maxBytes]
	return strings.TrimSpace(truncated) + "\n... (truncated)", true
}

func parseDocSpecifier(input string) (docid string, path string, line int) {
	input = strings.TrimSpace(input)
	if strings.HasPrefix(input, "#") {
		return strings.TrimPrefix(input, "#"), "", 0
	}
	if idx := strings.LastIndex(input, ":"); idx != -1 {
		pathPart := input[:idx]
		if n := parseLineNumber(input[idx+1:]); n > 0 {
			return "", pathPart, n
		}
	}
	return "", input, 0
}

func parseLineNumber(s string) int {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	if err != nil {
		return 0
	}
	return n
}

type docRecord struct {
	DocID      string
	Collection string
	RelPath    string
	Title      string
	Content    string
	Line       int
}

func (a *Adapter) fetchDocument(ctx context.Context, docid, path string) (*docRecord, error) {
	if docid != "" {
		doc, err := a.getDocByID(ctx, docid)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("document %q not found", "#"+docid)
		}
		return doc, err
	}
	doc, err := a.getDocByPath(ctx, path)
	if errors.Is(err, sql.ErrNoRows) {
		suggestions := suggestPaths(ctx, a.db, path)
		if len(suggestions) > 0 {
			return nil, fmt.Errorf("document not found. did you mean: %s", strings.Join(suggestions, ", "))
		}
		return nil, fmt.Errorf("document %q not found", path)
	}
	return doc, err
}

func (a *Adapter) getDocByID(ctx context.Context, docid string) (*docRecord, error) {
	var doc docRecord
	err := a.db.QueryRowContext(ctx, `SELECT docid, collection, relpath, title, content FROM documents WHERE docid = ?`, docid).
		Scan(&doc.DocID, &doc.Collection, &doc.RelPath, &doc.Title, &doc.Content)
	if err != nil {
		return nil, err
	}
	return &doc, nil
}

func (a *Adapter) getDocByPath(ctx context.Context, input string) (*docRecord, error) {
	input = strings.TrimPrefix(input, "./")
	parts := strings.SplitN(input, "/", 2)
	if len(parts) == 2 {
		if collectionExists(ctx, a.db, parts[0]) {
			return a.getDocByCollectionPath(ctx, parts[0], parts[1])
		}
	}
	return a.getDocByRelPath(ctx, input)
}

func (a *Adapter) getDocByCollectionPath(ctx context.Context, collection, relpath string) (*docRecord, error) {
	var doc docRecord
	err := a.db.QueryRowContext(ctx, `SELECT docid, collection, relpath, title, content FROM documents WHERE collection = ? AND relpath = ?`, collection, relpath).
		Scan(&doc.DocID, &doc.Collection, &doc.RelPath, &doc.Title, &doc.Content)
	if err != nil {
		return nil, err
	}
	return &doc, nil
}

func (a *Adapter) getDocByRelPath(ctx context.Context, relpath string) (*docRecord, error) {
	var doc docRecord
	err := a.db.QueryRowContext(ctx, `SELECT docid, collection, relpath, title, content FROM documents WHERE relpath = ?`, relpath).
		Scan(&doc.DocID, &doc.Collection, &doc.RelPath, &doc.Title, &doc.Content)
	if err != nil {
		return nil, err
	}
	return &doc, nil
}

func (a *Adapter) getDocBySuffix(ctx context.Context, suffix string) (*docRecord, error) {
	rows, err := a.db.QueryContext(ctx, `
		SELECT docid, collection, relpath, title, content
		FROM documents
		WHERE (collection || '/' || relpath) LIKE ?
		ORDER BY LENGTH(collection || '/' || relpath) ASC
		LIMIT 1`, "%"+suffix)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if rows.Next() {
		var doc docRecord
		if err := rows.Scan(&doc.DocID, &doc.Collection, &doc.RelPath, &doc.Title, &doc.Content); err != nil {
			return nil, err
		}
		return &doc, nil
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return nil, sql.ErrNoRows
}

func collectionExists(ctx context.Context, db *sql.DB, name string) bool {
	var exists int
	err := db.QueryRowContext(ctx, `SELECT 1 FROM collections WHERE name = ? LIMIT 1`, name).Scan(&exists)
	return err == nil
}

func (a *Adapter) fetchByGlob(ctx context.Context, pattern string) []docRecord {
	rows, err := a.db.QueryContext(ctx, `SELECT docid, collection, relpath, title, content FROM documents ORDER BY collection, relpath`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []docRecord
	for rows.Next() {
		var doc docRecord
		if err := rows.Scan(&doc.DocID, &doc.Collection, &doc.RelPath, &doc.Title, &doc.Content); err != nil {
			continue
		}
		path := doc.Collection + "/" + doc.RelPath
		ok, err := filepath.Match(pattern, path)
		if err != nil || !ok {
			continue
		}
		out = append(out, doc)
	}
	return out
}

func splitList(input string) []string {
	raw := strings.Split(input, ",")
	items := make([]string, 0, len(raw))
	for _, item := range raw {
		item = strings.TrimSpace(item)
		if item != "" {
			items = append(items, item)
		}
	}
	return items
}

func toContextPtr(ctxText string) *string {
	if ctxText == "" {
		return nil
	}
	return &ctxText
}

func (a *Adapter) lookupContext(ctx context.Context, collection, relpath string) string {
	ctxMap, err := a.loadContexts(ctx)
	if err != nil {
		return ""
	}
	return findContext(ctxMap, collection, relpath)
}

func (a *Adapter) loadContexts(ctx context.Context) (map[string]string, error) {
	rows, err := a.db.QueryContext(ctx, `SELECT virtual_path, description FROM path_contexts`)
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

func (a *Adapter) vectorAvailable(ctx context.Context) (bool, string, error) {
	if a.embedder == nil {
		return false, "embeddings not configured; set embed_model and run snip embed", nil
	}
	var count int
	if err := a.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM content_vectors`).Scan(&count); err != nil {
		return false, "", err
	}
	if count == 0 {
		return false, "vector index is empty; run snip embed", nil
	}
	return true, "", nil
}

func (a *Adapter) collectionStatus(ctx context.Context) ([]CollectionStatus, error) {
	rows, err := a.db.QueryContext(ctx, `
		SELECT c.name, c.path, c.mask, COUNT(d.docid), MAX(d.updated_at)
		FROM collections c
		LEFT JOIN documents d ON d.collection = c.name
		GROUP BY c.name
		ORDER BY c.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []CollectionStatus{}
	for rows.Next() {
		var (
			name       string
			path       string
			extensions sql.NullString
			docCount   int
			updatedRaw sql.NullInt64
		)
		if err := rows.Scan(&name, &path, &extensions, &docCount, &updatedRaw); err != nil {
			return nil, err
		}
		lastUpdated := ""
		if updatedRaw.Valid && updatedRaw.Int64 > 0 {
			lastUpdated = time.Unix(updatedRaw.Int64, 0).UTC().Format(time.RFC3339)
		}
		out = append(out, CollectionStatus{
			Name:        name,
			Path:        path,
			Extensions:  extensions.String,
			Documents:   docCount,
			LastUpdated: lastUpdated,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func suggestPaths(ctx context.Context, db *sql.DB, input string) []string {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil
	}
	rows, err := db.QueryContext(ctx, `SELECT collection, relpath FROM documents`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	type suggestion struct {
		path  string
		score int
	}
	var suggestions []suggestion
	for rows.Next() {
		var collection, relpath string
		if err := rows.Scan(&collection, &relpath); err != nil {
			continue
		}
		path := collection + "/" + relpath
		score := util.LevenshteinDistance(strings.ToLower(input), strings.ToLower(path))
		suggestions = append(suggestions, suggestion{path: path, score: score})
	}
	sort.Slice(suggestions, func(i, j int) bool { return suggestions[i].score < suggestions[j].score })
	out := []string{}
	for i := 0; i < len(suggestions) && i < 5; i++ {
		out = append(out, suggestions[i].path)
	}
	return out
}

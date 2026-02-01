package mcpserver

import (
	"database/sql"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"snip/internal/embed"
	"snip/internal/rerank"
	"snip/internal/search"
)

func New(db *sql.DB, embedder embed.Embedder, rr rerank.Reranker, expander search.Expander) *mcp.Server {
	adapter := NewAdapter(db, embedder, rr, expander)
	server := mcp.NewServer(&mcp.Implementation{Name: "snip", Version: "v1"}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "snip_search",
		Description: "Fast keyword search using BM25/FTS5.",
		InputSchema: searchInputSchema(0),
	}, adapter.Search)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "snip_vsearch",
		Description: "Semantic vector search (requires embeddings).",
		InputSchema: searchInputSchema(0.3),
	}, adapter.VSearch)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "snip_query",
		Description: "Hybrid search combining BM25 and vectors with reranking.",
		InputSchema: searchInputSchema(0),
	}, adapter.Query)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "snip_get",
		Description: "Retrieve a document by path or docid.",
		InputSchema: getInputSchema(),
	}, adapter.Get)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "snip_multi_get",
		Description: "Retrieve multiple documents by glob or list.",
		InputSchema: multiGetInputSchema(),
	}, adapter.MultiGet)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "snip_status",
		Description: "Return index status and collection stats.",
	}, adapter.Status)

	server.AddResourceTemplate(&mcp.ResourceTemplate{
		Name:        "snip_document",
		Title:       "SNIP document",
		Description: "Read a SNIP document by collection-relative path.",
		MIMEType:    "text/markdown",
		URITemplate: "snip://{+path}",
	}, adapter.ReadResource)

	server.AddPrompt(&mcp.Prompt{
		Name:        "query",
		Description: "Guidance for SNIP search and retrieval tools.",
	}, adapter.QueryPrompt)

	return server
}

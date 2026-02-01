package mcpserver

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"snip/internal/db"
	"snip/internal/embed"
	"snip/internal/rerank"
)

func TestServerRegistersTools(t *testing.T) {
	ctx := context.Background()
	sqlDB, _, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer sqlDB.Close()

	server := New(sqlDB, embed.NewHashEmbedder(8), rerank.LexicalReranker{}, nil)
	client := mcp.NewClient(&mcp.Implementation{Name: "client"}, nil)
	t1, t2 := mcp.NewInMemoryTransports()
	if _, err := server.Connect(ctx, t1, nil); err != nil {
		t.Fatalf("connect server: %v", err)
	}
	session, err := client.Connect(ctx, t2, nil)
	if err != nil {
		t.Fatalf("connect client: %v", err)
	}
	defer session.Close()

	res, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	got := map[string]bool{}
	for _, tool := range res.Tools {
		got[tool.Name] = true
	}
	for _, name := range []string{
		"snip_search",
		"snip_vsearch",
		"snip_query",
		"snip_get",
		"snip_multi_get",
		"snip_status",
	} {
		if !got[name] {
			t.Fatalf("missing tool %q", name)
		}
	}
}

func TestStatusStructuredContent(t *testing.T) {
	ctx := context.Background()
	sqlDB, _, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer sqlDB.Close()

	now := time.Now().Unix()
	if _, err := sqlDB.Exec(`INSERT INTO collections(name, path, mask) VALUES(?,?,?)`, "notes", "/tmp/notes", "*.md"); err != nil {
		t.Fatalf("insert collection: %v", err)
	}
	if _, err := sqlDB.Exec(`INSERT INTO documents(docid, collection, relpath, title, content, hash, updated_at) VALUES(?,?,?,?,?,?,?)`,
		"abc123", "notes", "a.md", "Title", "hello world", "hash1", now); err != nil {
		t.Fatalf("insert doc: %v", err)
	}

	server := New(sqlDB, embed.NewHashEmbedder(8), rerank.LexicalReranker{}, nil)
	client := mcp.NewClient(&mcp.Implementation{Name: "client"}, nil)
	t1, t2 := mcp.NewInMemoryTransports()
	if _, err := server.Connect(ctx, t1, nil); err != nil {
		t.Fatalf("connect server: %v", err)
	}
	session, err := client.Connect(ctx, t2, nil)
	if err != nil {
		t.Fatalf("connect client: %v", err)
	}
	defer session.Close()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "snip_status"})
	if err != nil {
		t.Fatalf("call snip_status: %v", err)
	}
	if result.StructuredContent == nil {
		t.Fatalf("expected structured content")
	}
	raw, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("marshal structured content: %v", err)
	}
	var out StatusOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal structured content: %v", err)
	}
	if out.TotalDocuments != 1 {
		t.Fatalf("total documents mismatch: %d", out.TotalDocuments)
	}
	if len(out.Collections) != 1 || out.Collections[0].Name != "notes" {
		t.Fatalf("collection status mismatch: %#v", out.Collections)
	}
}

package mcpserver

import (
	"strings"
	"testing"

	"snip/internal/search"
)

func TestParseDocSpecifier(t *testing.T) {
	docid, path, line := parseDocSpecifier("#abc123")
	if docid != "abc123" || path != "" || line != 0 {
		t.Fatalf("docid parse mismatch: %q %q %d", docid, path, line)
	}

	docid, path, line = parseDocSpecifier("notes/a.md:12")
	if docid != "" || path != "notes/a.md" || line != 12 {
		t.Fatalf("line parse mismatch: %q %q %d", docid, path, line)
	}

	docid, path, line = parseDocSpecifier("notes/a.md")
	if docid != "" || path != "notes/a.md" || line != 0 {
		t.Fatalf("path parse mismatch: %q %q %d", docid, path, line)
	}
}

func TestSnippetWithLines(t *testing.T) {
	content := "one\ntwo\nthree\nfour\nfive"
	snippet := snippetWithLines(content, "three")
	if !strings.Contains(snippet, "   3 | three") {
		t.Fatalf("expected line number in snippet: %q", snippet)
	}
	if !strings.Contains(snippet, "   2 | two") {
		t.Fatalf("expected context lines in snippet: %q", snippet)
	}
}

func TestFormatSearchResults(t *testing.T) {
	results := []search.Result{{
		DocID:      "abc123",
		Collection: "notes",
		RelPath:    "a.md",
		Title:      "Title",
		Content:    "hello\nworld",
		Score:      1.2,
		Context:    "ctx",
	}}
	out := formatSearchResults(results, "hello")
	if len(out) != 1 {
		t.Fatalf("expected 1 result, got %d", len(out))
	}
	if out[0].DocID != "#abc123" {
		t.Fatalf("docid not prefixed: %q", out[0].DocID)
	}
	if out[0].File != "notes/a.md" {
		t.Fatalf("file mismatch: %q", out[0].File)
	}
	if out[0].Score != 1.0 {
		t.Fatalf("score not clamped: %v", out[0].Score)
	}
	if out[0].Context == nil || *out[0].Context != "ctx" {
		t.Fatalf("context mismatch: %#v", out[0].Context)
	}
	if !strings.Contains(out[0].Snippet, "   1 | hello") {
		t.Fatalf("snippet missing line numbers: %q", out[0].Snippet)
	}
}

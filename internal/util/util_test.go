package util

import "testing"

func TestDocIDFromPath(t *testing.T) {
	id := DocIDFromPath("notes", "a/b.md")
	if len(id) != docIDLen {
		t.Fatalf("expected length %d, got %d", docIDLen, len(id))
	}
	if got := DocIDFromPath("notes", "a/b.md"); got != id {
		t.Fatalf("expected stable docid, got %s", got)
	}
	if other := DocIDFromPath("notes", "a/c.md"); other == id {
		t.Fatalf("expected different docid for different path")
	}
}

func TestTitleFromMarkdown(t *testing.T) {
	content := "# Hello World\n\ntext"
	if got := TitleFromMarkdown(content, "file.md"); got != "Hello World" {
		t.Fatalf("expected title, got %s", got)
	}
	content = "\n\n## Sub Title\ntext"
	if got := TitleFromMarkdown(content, "file.md"); got != "Sub Title" {
		t.Fatalf("expected subtitle, got %s", got)
	}
	content = "no heading here"
	if got := TitleFromMarkdown(content, "notes.md"); got != "notes" {
		t.Fatalf("expected fallback, got %s", got)
	}
}

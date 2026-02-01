package util

import "testing"

func TestDocIDFromHash(t *testing.T) {
	hash := "abcdef123456"
	if got := DocIDFromHash(hash); got != "abcdef" {
		t.Fatalf("expected abcdef, got %s", got)
	}
	short := "abc"
	if got := DocIDFromHash(short); got != short {
		t.Fatalf("expected %s, got %s", short, got)
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

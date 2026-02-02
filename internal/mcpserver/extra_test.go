package mcpserver

import "testing"

func TestEnforceMaxBytes(t *testing.T) {
	content, truncated := enforceMaxBytes("hello world", 5)
	if !truncated {
		t.Fatalf("expected truncation")
	}
	if content != "hello\n... (truncated)" {
		t.Fatalf("unexpected truncated content: %q", content)
	}
}

func TestIsGlobPattern(t *testing.T) {
	if !isGlobPattern("notes/*.md") {
		t.Fatalf("expected glob detection")
	}
	if isGlobPattern("notes/readme.md") {
		t.Fatalf("expected non-glob path")
	}
}

package cmd

import "testing"

func TestParseDocSpecifier(t *testing.T) {
	docid, path, line := parseDocSpecifier("#abc123")
	if docid != "abc123" || path != "" || line != 0 {
		t.Fatalf("docid parse mismatch: %q %q %d", docid, path, line)
	}

	docid, path, line = parseDocSpecifier("notes/a.md:12")
	if docid != "" || path != "notes/a.md" || line != 12 {
		t.Fatalf("line parse mismatch: %q %q %d", docid, path, line)
	}
}

func TestIsGlobPattern(t *testing.T) {
	if !isGlobPattern("notes/*.md") {
		t.Fatalf("expected glob pattern to be detected")
	}
	if isGlobPattern("notes/readme.md") {
		t.Fatalf("expected non-glob path")
	}
}

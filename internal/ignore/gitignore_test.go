package ignore

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMatcherRespectsGitignore(t *testing.T) {
	root := t.TempDir()
	gitignore := "*.log\n!important.log\nbuild/\n"
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(gitignore), 0o644); err != nil {
		t.Fatalf("write gitignore: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "build"), 0o755); err != nil {
		t.Fatalf("mkdir build: %v", err)
	}

	matcher := NewMatcher(root)
	if !matcher.Ignored(filepath.Join(root, "app.log"), false) {
		t.Fatalf("expected app.log ignored")
	}
	if matcher.Ignored(filepath.Join(root, "important.log"), false) {
		t.Fatalf("expected important.log not ignored")
	}
	if !matcher.Ignored(filepath.Join(root, "build"), true) {
		t.Fatalf("expected build directory ignored")
	}
}

func TestMatcherNestedGitignore(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("mkdir sub: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sub, ".gitignore"), []byte("skip.md\n"), 0o644); err != nil {
		t.Fatalf("write sub gitignore: %v", err)
	}

	matcher := NewMatcher(root)
	if !matcher.Ignored(filepath.Join(sub, "skip.md"), false) {
		t.Fatalf("expected nested ignore to apply")
	}
	if matcher.Ignored(filepath.Join(sub, "keep.md"), false) {
		t.Fatalf("expected keep.md not ignored")
	}
}

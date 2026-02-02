package output

import (
	"bytes"
	"strings"
	"testing"

	"snip/internal/search"
)

func TestWriteResultsLineNumbers(t *testing.T) {
	results := []search.Result{{
		DocID:      "abc123",
		Collection: "notes",
		RelPath:    "a.md",
		Title:      "Title",
		Content:    "one\ntwo",
		Score:      1.0,
	}}
	buf := &bytes.Buffer{}
	opts := Options{LineNumbers: true, NoColor: true}
	if err := WriteResults(buf, results, opts); err != nil {
		t.Fatalf("write error: %v", err)
	}
	if !strings.Contains(buf.String(), "1 | one") {
		t.Fatalf("expected line numbers in output: %q", buf.String())
	}
}

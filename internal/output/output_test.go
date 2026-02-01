package output

import (
	"bytes"
	"strings"
	"testing"

	"snip/internal/search"
)

func TestWriteResultsFormats(t *testing.T) {
	results := []search.Result{{DocID: "abc123", Collection: "notes", RelPath: "a.md", Title: "Title", Score: 0.9}}
	formats := []string{"json", "csv", "md", "xml"}
	for _, format := range formats {
		buf := &bytes.Buffer{}
		opts := Options{Format: format}
		if err := WriteResults(buf, results, opts); err != nil {
			t.Fatalf("format %s error: %v", format, err)
		}
		if buf.Len() == 0 {
			t.Fatalf("format %s output empty", format)
		}
	}

	buf := &bytes.Buffer{}
	opts := Options{FilesOnly: true}
	if err := WriteResults(buf, results, opts); err != nil {
		t.Fatalf("files output error: %v", err)
	}
	if !strings.Contains(buf.String(), "notes/a.md") {
		t.Fatalf("files output missing path")
	}
}

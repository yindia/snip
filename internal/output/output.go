package output

import (
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"strings"

	"snip/internal/search"
)

type Options struct {
	Format      string
	FilesOnly   bool
	NoColor     bool
	LineNumbers bool
}

func WriteResults(w io.Writer, results []search.Result, opts Options) error {
	if opts.FilesOnly {
		for _, r := range results {
			_, _ = fmt.Fprintf(w, "%s/%s\n", r.Collection, r.RelPath)
		}
		return nil
	}
	switch opts.Format {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(results)
	case "csv":
		return writeCSV(w, results)
	case "md":
		return writeMarkdown(w, results)
	case "xml":
		return writeXML(w, results)
	default:
		return writeHuman(w, results, opts)
	}
}

func writeHuman(w io.Writer, results []search.Result, opts Options) error {
	for i, r := range results {
		if i > 0 {
			_, _ = fmt.Fprintln(w, strings.Repeat("-", 60))
		}
		title := r.Title
		path := fmt.Sprintf("%s/%s", r.Collection, r.RelPath)
		_, _ = fmt.Fprintf(w, "%s  (score: %.4f)\n", colorize(title, "36;1", opts.NoColor), r.Score)
		_, _ = fmt.Fprintf(w, "%s  [%s]\n", colorize(path, "33", opts.NoColor), r.DocID)
		if r.Context != "" {
			_, _ = fmt.Fprintf(w, "context: %s\n", r.Context)
		}
		if r.Snippet != "" {
			snippet := r.Snippet
			if opts.LineNumbers {
				snippet = addLineNumbers(snippet, 1)
			}
			_, _ = fmt.Fprintf(w, "%s\n", snippet)
		}
		if r.Content != "" {
			content := r.Content
			if opts.LineNumbers {
				content = addLineNumbers(content, 1)
			}
			_, _ = fmt.Fprintf(w, "%s\n", content)
		}
	}
	return nil
}

func writeCSV(w io.Writer, results []search.Result) error {
	writer := csv.NewWriter(w)
	_ = writer.Write([]string{"docid", "collection", "relpath", "title", "score"})
	for _, r := range results {
		_ = writer.Write([]string{r.DocID, r.Collection, r.RelPath, r.Title, fmt.Sprintf("%.6f", r.Score)})
	}
	writer.Flush()
	return writer.Error()
}

func writeMarkdown(w io.Writer, results []search.Result) error {
	_, _ = fmt.Fprintln(w, "| docid | collection | relpath | title | score |")
	_, _ = fmt.Fprintln(w, "| --- | --- | --- | --- | --- |")
	for _, r := range results {
		_, _ = fmt.Fprintf(w, "| %s | %s | %s | %s | %.6f |\n", r.DocID, r.Collection, r.RelPath, escapeMarkdown(r.Title), r.Score)
	}
	return nil
}

func writeXML(w io.Writer, results []search.Result) error {
	type xmlResult struct {
		XMLName    xml.Name `xml:"result"`
		DocID      string   `xml:"docid"`
		Collection string   `xml:"collection"`
		RelPath    string   `xml:"relpath"`
		Title      string   `xml:"title"`
		Score      float64  `xml:"score"`
	}
	type wrapper struct {
		XMLName xml.Name    `xml:"results"`
		Items   []xmlResult `xml:"result"`
	}
	items := make([]xmlResult, 0, len(results))
	for _, r := range results {
		items = append(items, xmlResult{
			DocID: r.DocID, Collection: r.Collection, RelPath: r.RelPath, Title: r.Title, Score: r.Score,
		})
	}
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	return enc.Encode(wrapper{Items: items})
}

func colorize(s, code string, noColor bool) string {
	if noColor {
		return s
	}
	return "\x1b[" + code + "m" + s + "\x1b[0m"
}

func addLineNumbers(text string, start int) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = fmt.Sprintf("%4d | %s", start+i, line)
	}
	return strings.Join(lines, "\n")
}

func escapeMarkdown(s string) string {
	replacer := strings.NewReplacer("|", "\\|", "\n", " ")
	return replacer.Replace(s)
}

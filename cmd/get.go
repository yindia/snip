package cmd

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"snip/internal/util"
)

func getCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <path|#docid|path:line|glob|list>",
		Short: "Retrieve a document",
		Long:  "Fetch a document by path or docid, or multiple documents by glob/list.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			h, err := openDB()
			if err != nil {
				return err
			}
			defer h.DB.Close()
			input := strings.TrimSpace(args[0])
			if isGlobPattern(input) {
				return multiGetByGlob(cmd, h.DB, input)
			}
			if strings.Contains(input, ",") {
				return multiGetByList(cmd, h.DB, input)
			}
			docid, path, line := parseDocSpecifier(input)
			var doc *docRecord
			if docid != "" {
				doc, err = getDocByID(h.DB, docid)
			} else {
				doc, err = getDocByPath(h.DB, path)
			}
			if err != nil {
				suggestions := suggestPaths(h.DB, path)
				if len(suggestions) > 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "not found. did you mean:\n")
					for _, s := range suggestions {
						fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", s)
					}
					return nil
				}
				return err
			}
			if line > 0 {
				return printLine(cmd, doc.Content, line)
			}
			fmt.Fprintln(cmd.OutOrStdout(), doc.Content)
			return nil
		},
	}
}

type docRecord struct {
	DocID      string
	Collection string
	RelPath    string
	Title      string
	Content    string
}

func getDocByID(db *sql.DB, docid string) (*docRecord, error) {
	var doc docRecord
	err := db.QueryRow(`SELECT docid, collection, relpath, title, content FROM documents WHERE docid = ?`, docid).
		Scan(&doc.DocID, &doc.Collection, &doc.RelPath, &doc.Title, &doc.Content)
	if err != nil {
		return nil, err
	}
	return &doc, nil
}

func getDocByPath(db *sql.DB, input string) (*docRecord, error) {
	input = strings.TrimPrefix(input, "./")
	parts := strings.SplitN(input, "/", 2)
	if len(parts) == 2 {
		if collectionExists(db, parts[0]) {
			return getDocByCollectionPath(db, parts[0], parts[1])
		}
	}
	return getDocByRelPath(db, input)
}

func getDocByCollectionPath(db *sql.DB, collection, relpath string) (*docRecord, error) {
	var doc docRecord
	err := db.QueryRow(`SELECT docid, collection, relpath, title, content FROM documents WHERE collection = ? AND relpath = ?`, collection, relpath).
		Scan(&doc.DocID, &doc.Collection, &doc.RelPath, &doc.Title, &doc.Content)
	if err != nil {
		return nil, err
	}
	return &doc, nil
}

func getDocByRelPath(db *sql.DB, relpath string) (*docRecord, error) {
	var doc docRecord
	err := db.QueryRow(`SELECT docid, collection, relpath, title, content FROM documents WHERE relpath = ?`, relpath).
		Scan(&doc.DocID, &doc.Collection, &doc.RelPath, &doc.Title, &doc.Content)
	if err != nil {
		return nil, err
	}
	return &doc, nil
}

func collectionExists(db *sql.DB, name string) bool {
	var exists int
	err := db.QueryRow(`SELECT 1 FROM collections WHERE name = ? LIMIT 1`, name).Scan(&exists)
	return err == nil
}

func suggestPaths(db *sql.DB, input string) []string {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil
	}
	rows, err := db.Query(`SELECT collection, relpath FROM documents`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	type suggestion struct {
		path  string
		score int
	}
	var suggestions []suggestion
	for rows.Next() {
		var collection, relpath string
		if err := rows.Scan(&collection, &relpath); err != nil {
			continue
		}
		path := collection + "/" + relpath
		score := util.LevenshteinDistance(strings.ToLower(input), strings.ToLower(path))
		suggestions = append(suggestions, suggestion{path: path, score: score})
	}
	sort.Slice(suggestions, func(i, j int) bool { return suggestions[i].score < suggestions[j].score })
	out := []string{}
	for i := 0; i < len(suggestions) && i < 5; i++ {
		out = append(out, suggestions[i].path)
	}
	return out
}

func printLine(cmd *cobra.Command, content string, line int) error {
	if line <= 0 {
		fmt.Fprintln(cmd.OutOrStdout(), content)
		return nil
	}
	lines := strings.Split(content, "\n")
	if line > len(lines) {
		return fmt.Errorf("line %d out of range", line)
	}
	start := line - 2
	if start < 1 {
		start = 1
	}
	end := line + 2
	if end > len(lines) {
		end = len(lines)
	}
	for i := start; i <= end; i++ {
		prefix := "  "
		if i == line {
			prefix = "> "
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s%4d | %s\n", prefix, i, lines[i-1])
	}
	return nil
}

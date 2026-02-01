package cmd

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"snip/internal/indexer"
	"snip/internal/output"
)

type searchFlags struct {
	limit       int
	collection  string
	all         bool
	minScore    float64
	full        bool
	lineNumbers bool
	files       bool
	json        bool
	csv         bool
	md          bool
	xml         bool
}

func addSearchFlags(cmd *cobra.Command, flags *searchFlags) {
	cmd.Flags().IntVarP(&flags.limit, "limit", "n", 5, "number of results")
	cmd.Flags().StringVarP(&flags.collection, "collection", "c", "", "collection name")
	cmd.Flags().BoolVar(&flags.all, "all", false, "search all collections")
	cmd.Flags().Float64Var(&flags.minScore, "min-score", 0.0, "minimum score")
	cmd.Flags().BoolVar(&flags.full, "full", false, "include full content")
	cmd.Flags().BoolVar(&flags.lineNumbers, "line-numbers", false, "include line numbers")
	cmd.Flags().BoolVar(&flags.files, "files", false, "output file paths only")
	cmd.Flags().BoolVar(&flags.json, "json", false, "output json")
	cmd.Flags().BoolVar(&flags.csv, "csv", false, "output csv")
	cmd.Flags().BoolVar(&flags.md, "md", false, "output markdown")
	cmd.Flags().BoolVar(&flags.xml, "xml", false, "output xml")
}

func outputOptionsFromFlags(flags searchFlags) (output.Options, error) {
	format := ""
	count := 0
	if flags.json {
		format = "json"
		count++
	}
	if flags.csv {
		format = "csv"
		count++
	}
	if flags.md {
		format = "md"
		count++
	}
	if flags.xml {
		format = "xml"
		count++
	}
	if count > 1 {
		return output.Options{}, fmt.Errorf("choose only one output format")
	}
	return output.Options{Format: format, FilesOnly: flags.files, NoColor: cfg.NoColor, LineNumbers: flags.lineNumbers}, nil
}

func loadCollections(db *sql.DB) ([]indexer.Collection, error) {
	rows, err := db.Query(`SELECT name, path, mask FROM collections ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var cols []indexer.Collection
	for rows.Next() {
		var name, path, mask string
		if err := rows.Scan(&name, &path, &mask); err != nil {
			return nil, err
		}
		cols = append(cols, indexer.Collection{Name: name, Path: path, Mask: mask})
	}
	return cols, rows.Err()
}

func parseDocSpecifier(input string) (docid string, path string, line int) {
	input = strings.TrimSpace(input)
	if strings.HasPrefix(input, "#") {
		return strings.TrimPrefix(input, "#"), "", 0
	}
	line = 0
	if idx := strings.LastIndex(input, ":"); idx != -1 {
		pathPart := input[:idx]
		if n := parseLineNumber(input[idx+1:]); n > 0 {
			return "", pathPart, n
		}
	}
	return "", input, 0
}

func parseLineNumber(s string) int {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	if err != nil {
		return 0
	}
	return n
}

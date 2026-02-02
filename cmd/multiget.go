package cmd

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func multiGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:        "multi-get <glob|list>",
		Short:      "Retrieve multiple documents",
		Long:       "Fetch multiple documents by glob pattern or comma-separated list.",
		Deprecated: "use `snip get` with a glob pattern or comma-separated list",
		Args:       cobra.ExactArgs(1),
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
			return multiGetByList(cmd, h.DB, input)
		},
	}
}

func isGlobPattern(input string) bool {
	return strings.ContainsAny(input, "*?[]")
}

func multiGetByList(cmd *cobra.Command, db *sql.DB, input string) error {
	items := strings.Split(input, ",")
	for i, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		docid, path, line := parseDocSpecifier(item)
		var doc *docRecord
		var err error
		if docid != "" {
			doc, err = getDocByID(db, docid)
		} else {
			doc, err = getDocByPath(db, path)
		}
		if err != nil {
			return err
		}
		if i > 0 {
			fmt.Fprintln(cmd.OutOrStdout(), strings.Repeat("=", 60))
		}
		if line > 0 {
			if err := printLine(cmd, doc.Content, line); err != nil {
				return err
			}
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "%s/%s\n", doc.Collection, doc.RelPath)
			fmt.Fprintln(cmd.OutOrStdout(), doc.Content)
		}
	}
	return nil
}

func multiGetByGlob(cmd *cobra.Command, db *sql.DB, pattern string) error {
	rows, err := db.Query(`SELECT collection, relpath, content FROM documents ORDER BY collection, relpath`)
	if err != nil {
		return err
	}
	defer rows.Close()
	first := true
	for rows.Next() {
		var collection, relpath, content string
		if err := rows.Scan(&collection, &relpath, &content); err != nil {
			return err
		}
		path := collection + "/" + relpath
		ok, err := filepath.Match(pattern, path)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		if !first {
			fmt.Fprintln(cmd.OutOrStdout(), strings.Repeat("=", 60))
		}
		first = false
		fmt.Fprintf(cmd.OutOrStdout(), "%s\n", path)
		fmt.Fprintln(cmd.OutOrStdout(), content)
	}
	return rows.Err()
}

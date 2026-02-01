package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show index status",
		Long:  "Display index location, collection count, and indexed document stats.",
		RunE: func(cmd *cobra.Command, args []string) error {
			h, err := openDB()
			if err != nil {
				return err
			}
			defer h.DB.Close()
			var collections, docs, fts, vectors int
			_ = h.DB.QueryRow(`SELECT COUNT(*) FROM collections`).Scan(&collections)
			_ = h.DB.QueryRow(`SELECT COUNT(*) FROM documents`).Scan(&docs)
			_ = h.DB.QueryRow(`SELECT COUNT(*) FROM documents_fts`).Scan(&fts)
			_ = h.DB.QueryRow(`SELECT COUNT(*) FROM content_vectors`).Scan(&vectors)
			fmt.Fprintf(cmd.OutOrStdout(), "index: %s\n", h.Path)
			fmt.Fprintf(cmd.OutOrStdout(), "collections: %d\n", collections)
			fmt.Fprintf(cmd.OutOrStdout(), "documents: %d\n", docs)
			fmt.Fprintf(cmd.OutOrStdout(), "fts entries: %d\n", fts)
			fmt.Fprintf(cmd.OutOrStdout(), "vector chunks: %d\n", vectors)
			return nil
		},
	}
}

package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"snip/internal/indexer"
)

func updateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Index collections",
		Long:  "Scan collections, parse Markdown, and refresh the SQLite index.",
		RunE: func(cmd *cobra.Command, args []string) error {
			h, err := openDB()
			if err != nil {
				return err
			}
			defer h.DB.Close()
			collections, err := loadCollections(h.DB)
			if err != nil {
				return err
			}
			stats, err := indexer.Update(context.Background(), h.DB, collections)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "collections: %d, docs: %d, updated: %d, skipped: %d, removed: %d, errors: %d\n",
				stats.Collections, stats.Documents, stats.Updated, stats.Skipped, stats.Removed, stats.Errors)
			return nil
		},
	}
}

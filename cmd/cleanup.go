package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func cleanupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cleanup",
		Short: "Remove stale index data",
		Long:  "Delete orphaned vectors and FTS rows that no longer map to documents.",
		RunE: func(cmd *cobra.Command, args []string) error {
			h, err := openDB()
			if err != nil {
				return err
			}
			defer h.DB.Close()
			res1, err := h.DB.Exec(`DELETE FROM content_vectors WHERE hash NOT IN (SELECT hash FROM documents)`)
			if err != nil {
				return err
			}
			res2, err := h.DB.Exec(`DELETE FROM documents_fts WHERE docid NOT IN (SELECT docid FROM documents)`)
			if err != nil {
				return err
			}
			vecRemoved, _ := res1.RowsAffected()
			ftsRemoved, _ := res2.RowsAffected()
			fmt.Fprintf(cmd.OutOrStdout(), "removed vectors: %d, removed fts rows: %d\n", vecRemoved, ftsRemoved)
			return nil
		},
	}
}

package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"snip/internal/embed"
)

func embedCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "embed",
		Short: "Generate embeddings",
		Long:  "Chunk documents and generate deterministic embeddings for vector search.",
		RunE: func(cmd *cobra.Command, args []string) error {
			h, err := openDB()
			if err != nil {
				return err
			}
			defer h.DB.Close()
			embedder := newEmbedder()
			stats, err := embed.EmbedDocuments(context.Background(), h.DB, embedder, force)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "documents: %d, embedded: %d, skipped: %d, errors: %d\n", stats.Documents, stats.Embedded, stats.Skipped, stats.Errors)
			return nil
		},
	}
	cmd.Flags().BoolVarP(&force, "force", "f", false, "force re-embedding")
	return cmd
}

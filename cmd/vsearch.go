package cmd

import (
	"context"

	"github.com/spf13/cobra"

	"snip/internal/output"
	"snip/internal/search"
)

func vsearchCmd() *cobra.Command {
	flags := searchFlags{}
	cmd := &cobra.Command{
		Use:   "vsearch <query>",
		Short: "Vector search (embeddings)",
		Long:  "Run semantic search using document embeddings.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			h, err := openDB()
			if err != nil {
				return err
			}
			defer h.DB.Close()
			outOpts, err := outputOptionsFromFlags(flags)
			if err != nil {
				return err
			}
			embedder := newEmbedder()
			opts := search.Options{
				Limit:      flags.limit,
				Collection: flags.collection,
				All:        flags.all || flags.collection == "",
				MinScore:   flags.minScore,
				Full:       flags.full,
			}
			results, err := search.VectorSearch(context.Background(), h.DB, embedder, args[0], opts)
			if err != nil {
				return err
			}
			return output.WriteResults(cmd.OutOrStdout(), results, outOpts)
		},
	}
	addSearchFlags(cmd, &flags)
	return cmd
}

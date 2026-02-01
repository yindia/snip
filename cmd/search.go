package cmd

import (
	"context"

	"github.com/spf13/cobra"

	"snip/internal/output"
	"snip/internal/search"
)

func searchCmd() *cobra.Command {
	flags := searchFlags{}
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Keyword search (FTS)",
		Long:  "Run a BM25 full-text search over indexed documents.",
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
			opts := search.Options{
				Limit:      flags.limit,
				Collection: flags.collection,
				All:        flags.all || flags.collection == "",
				MinScore:   flags.minScore,
				Full:       flags.full,
			}
			results, err := search.FTSSearch(context.Background(), h.DB, args[0], opts)
			if err != nil {
				return err
			}
			return output.WriteResults(cmd.OutOrStdout(), results, outOpts)
		},
	}
	addSearchFlags(cmd, &flags)
	return cmd
}

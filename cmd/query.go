package cmd

import (
	"context"

	"github.com/spf13/cobra"

	"snip/internal/output"
	"snip/internal/rerank"
	"snip/internal/search"
)

func queryCmd() *cobra.Command {
	flags := searchFlags{}
	cmd := &cobra.Command{
		Use:   "query <query>",
		Short: "Hybrid search",
		Long:  "Run hybrid search that fuses BM25 and vector retrieval with reranking.",
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
			reranker := selectReranker()
			expander := selectExpander()
			opts := search.Options{
				Limit:      flags.limit,
				Collection: flags.collection,
				All:        flags.all || flags.collection == "",
				MinScore:   flags.minScore,
				Full:       flags.full,
			}
			results, err := search.HybridSearch(context.Background(), h.DB, embedder, reranker, expander, args[0], opts)
			if err != nil {
				return err
			}
			return output.WriteResults(cmd.OutOrStdout(), results, outOpts)
		},
	}
	addSearchFlags(cmd, &flags)
	return cmd
}

func selectReranker() rerank.Reranker {
	fallback := rerank.LexicalReranker{}
	backend, err := getLLMBackend()
	if err != nil || backend == nil || !backend.CanRerank() {
		return fallback
	}
	rr := rerank.NewYzmaReranker(backend)
	return rerank.FallbackReranker{Primary: rr, Fallback: fallback}
}

func selectExpander() search.Expander {
	backend, err := getLLMBackend()
	if err != nil || backend == nil || !backend.CanExpand() {
		return nil
	}
	return backend
}

package cmd

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"

	"snip/internal/mcpserver"
)

func mcpCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mcp",
		Short: "Start MCP server over stdio",
		Long:  "Start the SNIP MCP server over STDIO for agent integrations.",
		RunE: func(cmd *cobra.Command, args []string) error {
			h, err := openDB()
			if err != nil {
				return err
			}
			defer h.DB.Close()
			embedder := newEmbedder()
			reranker := selectReranker()
			expander := selectExpander()
			server := mcpserver.New(h.DB, embedder, reranker, expander)
			return server.Run(context.Background(), &mcp.StdioTransport{})
		},
	}
}

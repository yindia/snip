package cmd

import "github.com/spf13/cobra"

func lsCmd() *cobra.Command {
	cmd := collectionLsCmd()
	cmd.Use = "ls <collection> [subpath]"
	cmd.Short = "List documents in a collection"
	cmd.Long = "Alias for collection ls to list indexed documents in a collection."
	return cmd
}

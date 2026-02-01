package cmd

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"snip/internal/util"
)

func contextCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "context",
		Short: "Manage context annotations",
		Long:  "Contexts attach descriptions to paths or collections to improve display and reranking.",
	}
	cmd.AddCommand(contextAddCmd())
	cmd.AddCommand(contextListCmd())
	cmd.AddCommand(contextRemoveCmd())
	return cmd
}

func contextAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <path_or_snip_virtual> <description>",
		Short: "Add context for a path",
		Long:  "Attach a description to a path or snip:// virtual path for better results.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			h, err := openDB()
			if err != nil {
				return err
			}
			defer h.DB.Close()
			vp, err := resolveContextPath(h.DB, args[0])
			if err != nil {
				return err
			}
			desc := args[1]
			_, err = h.DB.Exec(`INSERT OR REPLACE INTO path_contexts(virtual_path, description) VALUES(?,?)`, vp, desc)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "added context %s\n", vp)
			return nil
		},
	}
}

func contextListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List contexts",
		Long:  "Show all stored context descriptions and their paths.",
		RunE: func(cmd *cobra.Command, args []string) error {
			h, err := openDB()
			if err != nil {
				return err
			}
			defer h.DB.Close()
			rows, err := h.DB.Query(`SELECT virtual_path, description FROM path_contexts ORDER BY virtual_path`)
			if err != nil {
				return err
			}
			defer rows.Close()
			for rows.Next() {
				var path, desc string
				if err := rows.Scan(&path, &desc); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\n", path, desc)
			}
			return rows.Err()
		},
	}
}

func contextRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <path_or_snip_virtual>",
		Short: "Remove context for a path",
		Long:  "Delete a context description for a path or snip:// virtual path.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			h, err := openDB()
			if err != nil {
				return err
			}
			defer h.DB.Close()
			vp, err := resolveContextPath(h.DB, args[0])
			if err != nil {
				return err
			}
			res, err := h.DB.Exec(`DELETE FROM path_contexts WHERE virtual_path = ?`, vp)
			if err != nil {
				return err
			}
			if count, _ := res.RowsAffected(); count == 0 {
				return fmt.Errorf("context not found: %s", vp)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "removed context %s\n", vp)
			return nil
		},
	}
}

func resolveContextPath(db *sql.DB, input string) (string, error) {
	if strings.HasPrefix(input, "snip://") {
		return input, nil
	}
	abs, err := util.CleanAbs(input)
	if err != nil {
		return "", err
	}
	rows, err := db.Query(`SELECT name, path FROM collections`)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	for rows.Next() {
		var name, root string
		if err := rows.Scan(&name, &root); err != nil {
			return "", err
		}
		root = filepath.Clean(root)
		if strings.HasPrefix(abs, root) {
			rel, err := filepath.Rel(root, abs)
			if err != nil {
				return "", err
			}
			rel = filepath.ToSlash(rel)
			if rel == "." || rel == "" {
				return "snip://" + name, nil
			}
			return "snip://" + name + "/" + rel, nil
		}
	}
	return "file://" + abs, nil
}

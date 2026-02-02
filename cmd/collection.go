package cmd

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"snip/internal/util"
)

func collectionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "collection",
		Short: "Manage collections",
		Long:  "Collections are named roots on disk that SNIP indexes and searches.",
	}
	cmd.AddCommand(collectionAddCmd())
	cmd.AddCommand(collectionListCmd())
	cmd.AddCommand(collectionRemoveCmd())
	cmd.AddCommand(collectionRenameCmd())
	cmd.AddCommand(collectionLsCmd())
	return cmd
}

func collectionAddCmd() *cobra.Command {
	var name string
	var extensions []string
	cmd := &cobra.Command{
		Use:   "add <path>",
		Short: "Add a collection",
		Long:  "Register a named collection at a local path for indexing.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" {
				return fmt.Errorf("--name is required")
			}
			path, err := util.CleanAbs(args[0])
			if err != nil {
				return err
			}
			info, err := os.Stat(path)
			if err != nil {
				return err
			}
			if !info.IsDir() {
				return fmt.Errorf("path is not a directory: %s", path)
			}
			h, err := openDB()
			if err != nil {
				return err
			}
			defer h.DB.Close()
			var existingPath, existingExtensions string
			err = h.DB.QueryRow(`SELECT path, mask FROM collections WHERE name = ?`, name).Scan(&existingPath, &existingExtensions)
			if err == nil {
				return fmt.Errorf("collection %q already exists (path: %s). use `snip collection rename %s <new>` or `snip collection remove %s`", name, existingPath, name, name)
			}
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return err
			}
			var existingName string
			err = h.DB.QueryRow(`SELECT name FROM collections WHERE path = ?`, path).Scan(&existingName)
			if err == nil && existingName != name {
				return fmt.Errorf("path %q is already registered as collection %q", path, existingName)
			}
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return err
			}
			extensionsValue := joinExtensions(extensions)
			_, err = h.DB.Exec(`INSERT INTO collections(name, path, mask) VALUES(?,?,?)`, name, path, extensionsValue)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "added collection %s (%s)\n", name, path)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "collection name")
	cmd.Flags().StringArrayVar(&extensions, "extension", nil, "file extension (repeatable; matches recursively; e.g. --extension go)")
	return cmd
}

func collectionListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List collections",
		Long:  "Show all registered collections with their paths and extensions.",
		RunE: func(cmd *cobra.Command, args []string) error {
			h, err := openDB()
			if err != nil {
				return err
			}
			defer h.DB.Close()
			rows, err := h.DB.Query(`SELECT name, path, mask FROM collections ORDER BY name`)
			if err != nil {
				return err
			}
			defer rows.Close()
			for rows.Next() {
				var name, path, extensions string
				if err := rows.Scan(&name, &path, &extensions); err != nil {
					return err
				}
				if extensions != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", name, path, extensions)
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\n", name, path)
				}
			}
			return rows.Err()
		},
	}
}

func collectionRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a collection",
		Long:  "Remove a collection and delete its indexed documents and vectors.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			h, err := openDB()
			if err != nil {
				return err
			}
			defer h.DB.Close()
			ctx := context.Background()
			tx, err := h.DB.BeginTx(ctx, nil)
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `
				DELETE FROM documents_fts
				WHERE docid IN (SELECT docid FROM documents WHERE collection = ?)`, name); err != nil {
				_ = tx.Rollback()
				return err
			}
			if _, err := tx.ExecContext(ctx, `
				DELETE FROM content_vectors
				WHERE hash IN (SELECT hash FROM documents WHERE collection = ?)`, name); err != nil {
				_ = tx.Rollback()
				return err
			}
			if _, err := tx.ExecContext(ctx, `DELETE FROM documents WHERE collection = ?`, name); err != nil {
				_ = tx.Rollback()
				return err
			}
			res, err := tx.ExecContext(ctx, `DELETE FROM collections WHERE name = ?`, name)
			if err != nil {
				_ = tx.Rollback()
				return err
			}
			if count, _ := res.RowsAffected(); count == 0 {
				_ = tx.Rollback()
				return fmt.Errorf("collection not found: %s", name)
			}
			if err := tx.Commit(); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "removed collection %s\n", name)
			return nil
		},
	}
}

func collectionRenameCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rename <old> <new>",
		Short: "Rename a collection",
		Long:  "Rename a collection and update document metadata and contexts.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			oldName := args[0]
			newName := args[1]
			h, err := openDB()
			if err != nil {
				return err
			}
			defer h.DB.Close()
			ctx := context.Background()
			tx, err := h.DB.BeginTx(ctx, nil)
			if err != nil {
				return err
			}
			_, err = tx.ExecContext(ctx, `UPDATE collections SET name = ? WHERE name = ?`, newName, oldName)
			if err != nil {
				_ = tx.Rollback()
				return err
			}
			_, err = tx.ExecContext(ctx, `UPDATE documents SET collection = ? WHERE collection = ?`, newName, oldName)
			if err != nil {
				_ = tx.Rollback()
				return err
			}
			rows, err := tx.QueryContext(ctx, `SELECT virtual_path, description FROM path_contexts`)
			if err != nil {
				_ = tx.Rollback()
				return err
			}
			for rows.Next() {
				var vp, desc string
				if err := rows.Scan(&vp, &desc); err != nil {
					rows.Close()
					_ = tx.Rollback()
					return err
				}
				if strings.HasPrefix(vp, "snip://"+oldName) {
					newVP := strings.Replace(vp, "snip://"+oldName, "snip://"+newName, 1)
					if _, err := tx.ExecContext(ctx, `UPDATE path_contexts SET virtual_path = ? WHERE virtual_path = ?`, newVP, vp); err != nil {
						rows.Close()
						_ = tx.Rollback()
						return err
					}
				}
			}
			rows.Close()
			if err := tx.Commit(); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "renamed collection %s -> %s\n", oldName, newName)
			return nil
		},
	}
}

func collectionLsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ls <collection> [subpath]",
		Short: "List documents in a collection",
		Long:  "List indexed documents under a collection and optional subpath prefix.",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			collection := args[0]
			prefix := ""
			if len(args) == 2 {
				prefix = strings.Trim(strings.TrimSpace(args[1]), "/")
			}
			h, err := openDB()
			if err != nil {
				return err
			}
			defer h.DB.Close()
			query := `SELECT relpath, title, docid FROM documents WHERE collection = ?`
			var rows *sql.Rows
			if prefix != "" {
				query = query + " AND relpath LIKE ? ORDER BY relpath"
				rows, err = h.DB.Query(query, collection, filepath.ToSlash(prefix)+"%")
			} else {
				query = query + " ORDER BY relpath"
				rows, err = h.DB.Query(query, collection)
			}
			if err != nil {
				return err
			}
			defer rows.Close()
			for rows.Next() {
				var relpath, title, docid string
				if err := rows.Scan(&relpath, &title, &docid); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t#%s\n", relpath, title, docid)
			}
			return rows.Err()
		},
	}
}

func collectionByName(db *sql.DB, name string) (string, error) {
	var path string
	err := db.QueryRow(`SELECT path FROM collections WHERE name = ?`, name).Scan(&path)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", fmt.Errorf("collection not found: %s", name)
		}
		return "", err
	}
	return path, nil
}

func joinExtensions(items []string) string {
	if len(items) == 0 {
		return ""
	}
	parts := []string{}
	for _, item := range items {
		for _, part := range strings.Split(item, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			if strings.ContainsAny(part, "*?[]/") {
				parts = append(parts, part)
				continue
			}
			part = strings.TrimPrefix(part, ".")
			part = strings.ToLower(part)
			if part != "" {
				parts = append(parts, part)
			}
		}
	}
	return strings.Join(parts, ",")
}

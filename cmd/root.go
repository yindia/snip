package cmd

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"snip/internal/config"
	"snip/internal/db"
	"snip/internal/embed"
	"snip/internal/util"
)

var (
	rootCmd = &cobra.Command{
		Use:   "snip",
		Short: "SNIP: Search, Navigate, Index, Parse",
		Long:  "SNIP is a local-first CLI search engine for Markdown notes and docs, with BM25, vector search, and a hybrid pipeline.",
	}
	cfgFile      string
	indexDirFlag string
	debugFlag    bool
	noColorFlag  bool
	cfg          config.Config
)

func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (yaml or json)")
	rootCmd.PersistentFlags().StringVar(&indexDirFlag, "index", "", "index directory")
	rootCmd.PersistentFlags().BoolVar(&debugFlag, "debug", false, "enable debug logging")
	rootCmd.PersistentFlags().BoolVar(&noColorFlag, "no-color", false, "disable color output")

	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		configPath := cfgFile
		if configPath == "" {
			configPath = defaultConfigPath()
		}
		loaded, err := config.Load(configPath)
		if err != nil {
			return err
		}
		cfg = loaded
		if indexDirFlag != "" {
			cfg.IndexDir = indexDirFlag
		}
		if debugFlag {
			cfg.Debug = true
		}
		if noColorFlag {
			cfg.NoColor = true
		}
		util.SetDebug(cfg.Debug)
		return nil
	}

	rootCmd.AddCommand(collectionCmd())
	rootCmd.AddCommand(lsCmd())
	rootCmd.AddCommand(contextCmd())
	rootCmd.AddCommand(updateCmd())
	rootCmd.AddCommand(statusCmd())
	rootCmd.AddCommand(cleanupCmd())
	rootCmd.AddCommand(embedCmd())
	rootCmd.AddCommand(searchCmd())
	rootCmd.AddCommand(vsearchCmd())
	rootCmd.AddCommand(queryCmd())
	rootCmd.AddCommand(getCmd())
	rootCmd.AddCommand(multiGetCmd())
}

func openDB() (*dbHandle, error) {
	sqlDB, path, err := db.Open(cfg.IndexDir)
	if err != nil {
		return nil, err
	}
	return &dbHandle{DB: sqlDB, Path: path}, nil
}

type dbHandle struct {
	DB   *sql.DB
	Path string
}

func defaultConfigPath() string {
	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		if dir, err := os.UserConfigDir(); err == nil {
			configDir = dir
		}
	}
	if configDir == "" {
		return ""
	}
	candidates := []string{
		filepath.Join(configDir, "snip", "config.yaml"),
		filepath.Join(configDir, "snip", "config.yml"),
		filepath.Join(configDir, "snip", "config.json"),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}

func newEmbedder() embed.Embedder {
	modelDir := filepath.Join(util.CacheDir(), "snip", "models")
	_ = os.MkdirAll(modelDir, 0o755)
	if cfg.Model == "" || cfg.Model == "hash" {
		return embed.NewHashEmbedder(256)
	}
	fmt.Fprintf(os.Stderr, "unknown model %q, falling back to hash\n", cfg.Model)
	return embed.NewHashEmbedder(256)
}

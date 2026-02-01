package cmd

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/spf13/cobra"

	"snip/internal/config"
	"snip/internal/db"
	"snip/internal/embed"
	"snip/internal/llm"
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
	llmOnce      sync.Once
	llmBackend   *llm.Backend
	llmErr       error
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
		applyIndexHint(configPath)
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
	rootCmd.AddCommand(mcpCmd())
}

func openDB() (*dbHandle, error) {
	sqlDB, path, err := db.Open(cfg.IndexDir)
	if err != nil {
		return nil, err
	}
	writeIndexHint(cfg.IndexDir)
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

func applyIndexHint(configPath string) {
	if indexDirFlag != "" {
		return
	}
	if configPath != "" {
		return
	}
	hint, err := readIndexHint()
	if err != nil || hint == "" {
		return
	}
	cfg.IndexDir = hint
}

func readIndexHint() (string, error) {
	paths := indexHintPaths()
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		hint := strings.TrimSpace(string(data))
		if hint != "" {
			return hint, nil
		}
	}
	return "", os.ErrNotExist
}

func writeIndexHint(indexDir string) {
	if indexDir == "" {
		return
	}
	for _, path := range indexHintPaths() {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			continue
		}
		_ = os.WriteFile(path, []byte(indexDir+"\n"), 0o644)
	}
}

func indexHintPaths() []string {
	paths := []string{}
	add := func(path string) {
		for _, existing := range paths {
			if existing == path {
				return
			}
		}
		paths = append(paths, path)
	}
	add(filepath.Join(util.CacheDir(), "snip", "last_index"))
	if userCache, err := os.UserCacheDir(); err == nil && userCache != "" {
		add(filepath.Join(userCache, "snip", "last_index"))
	}
	return paths
}

func newEmbedder() embed.Embedder {
	if cfg.EmbedModel == "" || cfg.EmbedModel == "hash" {
		return embed.NewHashEmbedder(256)
	}
	backend, err := getLLMBackend()
	if err != nil || backend == nil {
		util.Debugf("llm backend unavailable: %v", err)
		return embed.NewHashEmbedder(256)
	}
	return embed.NewYzmaEmbedder(backend)
}

func getLLMBackend() (*llm.Backend, error) {
	llmOnce.Do(func() {
		llmBackend, llmErr = llm.NewYzmaBackend(cfg)
	})
	return llmBackend, llmErr
}

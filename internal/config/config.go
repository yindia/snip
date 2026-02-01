package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"snip/internal/util"
)

type Config struct {
	IndexDir      string `json:"index_dir" yaml:"index_dir"`
	Model         string `json:"model" yaml:"model"`
	EmbedModel    string `json:"embed_model" yaml:"embed_model"`
	RerankModel   string `json:"rerank_model" yaml:"rerank_model"`
	ExpandModel   string `json:"expand_model" yaml:"expand_model"`
	ModelCacheDir string `json:"model_cache_dir" yaml:"model_cache_dir"`
	LlamaLibPath  string `json:"llama_lib_path" yaml:"llama_lib_path"`
	Debug         bool   `json:"debug" yaml:"debug"`
	NoColor       bool   `json:"no_color" yaml:"no_color"`
}

func Default() Config {
	return Config{
		IndexDir:      filepath.Join(util.CacheDir(), "snip"),
		Model:         "",
		EmbedModel:    "hf:ggml-org/embeddinggemma-300M-GGUF/embeddinggemma-300M-Q8_0.gguf",
		RerankModel:   "hf:ggml-org/Qwen3-Reranker-0.6B-Q8_0-GGUF/qwen3-reranker-0.6b-q8_0.gguf",
		ExpandModel:   "hf:tobil/qmd-query-expansion-1.7B-gguf/qmd-query-expansion-1.7B-q4_k_m.gguf",
		ModelCacheDir: filepath.Join(util.CacheDir(), "snip", "models"),
		LlamaLibPath:  filepath.Join(util.CacheDir(), "snip", "llama"),
		Debug:         false,
		NoColor:       false,
	}
}

func Load(path string) (Config, error) {
	cfg := Default()
	if path == "" {
		return ApplyEnv(cfg), nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ApplyEnv(cfg), nil
		}
		return cfg, err
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".json":
		if err := json.Unmarshal(data, &cfg); err != nil {
			return cfg, err
		}
	default:
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return cfg, err
		}
	}
	cfg = ApplyEnv(cfg)
	return cfg, nil
}

func ApplyEnv(cfg Config) Config {
	if v := os.Getenv("SNIP_INDEX_DIR"); v != "" {
		cfg.IndexDir = v
	}
	if v := os.Getenv("SNIP_MODEL"); v != "" {
		cfg.Model = v
	}
	if v := os.Getenv("SNIP_EMBED_MODEL"); v != "" {
		cfg.EmbedModel = v
	}
	if v := os.Getenv("SNIP_RERANK_MODEL"); v != "" {
		cfg.RerankModel = v
	}
	if v := os.Getenv("SNIP_EXPAND_MODEL"); v != "" {
		cfg.ExpandModel = v
	}
	if v := os.Getenv("SNIP_MODEL_CACHE_DIR"); v != "" {
		cfg.ModelCacheDir = v
	}
	if v := os.Getenv("SNIP_LLAMA_LIB"); v != "" {
		cfg.LlamaLibPath = v
	}
	if v := os.Getenv("YZMA_LIB"); v != "" && cfg.LlamaLibPath == "" {
		cfg.LlamaLibPath = v
	}
	if v := os.Getenv("SNIP_DEBUG"); v != "" {
		cfg.Debug = parseBool(v)
	}
	if cfg.EmbedModel == "" && cfg.Model != "" {
		cfg.EmbedModel = cfg.Model
	}
	return cfg
}

func parseBool(v string) bool {
	v = strings.TrimSpace(strings.ToLower(v))
	switch v {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}

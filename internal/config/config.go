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
	IndexDir string `json:"index_dir" yaml:"index_dir"`
	Model    string `json:"model" yaml:"model"`
	Debug    bool   `json:"debug" yaml:"debug"`
	NoColor  bool   `json:"no_color" yaml:"no_color"`
}

func Default() Config {
	return Config{
		IndexDir: filepath.Join(util.CacheDir(), "snip"),
		Model:    "hash",
		Debug:    false,
		NoColor:  false,
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
	if v := os.Getenv("SNIP_DEBUG"); v != "" {
		cfg.Debug = parseBool(v)
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

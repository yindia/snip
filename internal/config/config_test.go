package config

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := Default()
	if cfg.IndexDir == "" {
		t.Fatalf("expected IndexDir to be set")
	}
	if cfg.ModelCacheDir == "" || !strings.Contains(cfg.ModelCacheDir, "snip") {
		t.Fatalf("expected ModelCacheDir to include snip, got %q", cfg.ModelCacheDir)
	}
	if filepath.Base(cfg.ModelCacheDir) != "models" {
		t.Fatalf("expected ModelCacheDir to end with models, got %q", cfg.ModelCacheDir)
	}
	if cfg.LlamaLibPath == "" || !strings.Contains(cfg.LlamaLibPath, "snip") {
		t.Fatalf("expected LlamaLibPath to include snip, got %q", cfg.LlamaLibPath)
	}
	if cfg.EmbedModel == "" {
		t.Fatalf("expected EmbedModel default")
	}
}

func TestApplyEnvOverrides(t *testing.T) {
	t.Setenv("SNIP_INDEX_DIR", "/tmp/snip-index")
	t.Setenv("SNIP_DEBUG", "true")
	t.Setenv("SNIP_MODEL", "alias-model")

	cfg := ApplyEnv(Config{})
	if cfg.IndexDir != "/tmp/snip-index" {
		t.Fatalf("expected IndexDir override, got %q", cfg.IndexDir)
	}
	if !cfg.Debug {
		t.Fatalf("expected Debug enabled")
	}
	if cfg.EmbedModel != "alias-model" {
		t.Fatalf("expected EmbedModel to use legacy alias, got %q", cfg.EmbedModel)
	}
}

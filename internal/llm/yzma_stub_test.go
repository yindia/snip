//go:build !yzma
// +build !yzma

package llm

import (
	"testing"

	"snip/internal/config"
)

func TestYzmaStubBackend(t *testing.T) {
	if _, err := NewYzmaBackend(config.Config{}); err == nil {
		t.Fatalf("expected error when yzma support not built")
	}
	b := &Backend{}
	if b.CanRerank() || b.CanExpand() {
		t.Fatalf("expected stub backend to report no capabilities")
	}
	if _, err := b.Embed([]string{"test"}); err == nil {
		t.Fatalf("expected embed to return error")
	}
}

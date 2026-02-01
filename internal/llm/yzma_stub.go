//go:build !yzma
// +build !yzma

package llm

import (
	"errors"

	"snip/internal/config"
)

type Backend struct{}

type Doc struct {
	Title   string
	Content string
	Context string
}

func NewYzmaBackend(cfg config.Config) (*Backend, error) {
	return nil, errors.New("yzma support not built (use -tags yzma)")
}

func (b *Backend) Embed(texts []string) ([][]float32, error) {
	return nil, errors.New("yzma support not built")
}

func (b *Backend) EmbedDim() int {
	return 0
}

func (b *Backend) CanRerank() bool {
	return false
}

func (b *Backend) CanExpand() bool {
	return false
}

func (b *Backend) FormatQuery(query string) string {
	return query
}

func (b *Backend) FormatDocument(title, text string) string {
	return text
}

func (b *Backend) Rerank(query string, docs []Doc) ([]float64, error) {
	return nil, errors.New("yzma support not built")
}

func (b *Backend) Expand(query string) ([]string, error) {
	return nil, errors.New("yzma support not built")
}

package embed

import "snip/internal/llm"

type YzmaEmbedder struct {
	backend *llm.Backend
}

func NewYzmaEmbedder(backend *llm.Backend) *YzmaEmbedder {
	return &YzmaEmbedder{backend: backend}
}

func (y *YzmaEmbedder) Name() string {
	return "yzma"
}

func (y *YzmaEmbedder) Dim() int {
	if y.backend == nil {
		return 0
	}
	return y.backend.EmbedDim()
}

func (y *YzmaEmbedder) Embed(texts []string) ([][]float32, error) {
	if y.backend == nil {
		return nil, nil
	}
	return y.backend.Embed(texts)
}

func (y *YzmaEmbedder) FormatQuery(query string) string {
	if y.backend == nil {
		return query
	}
	return y.backend.FormatQuery(query)
}

func (y *YzmaEmbedder) FormatDocument(title, text string) string {
	if y.backend == nil {
		return text
	}
	return y.backend.FormatDocument(title, text)
}

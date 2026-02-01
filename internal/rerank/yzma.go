package rerank

import (
	"errors"

	"snip/internal/llm"
)

type YzmaReranker struct {
	backend *llm.Backend
}

func NewYzmaReranker(backend *llm.Backend) *YzmaReranker {
	return &YzmaReranker{backend: backend}
}

func (y *YzmaReranker) Rerank(query string, docs []Doc) ([]float64, error) {
	if y.backend == nil {
		return nil, errors.New("llm backend not available")
	}
	llmDocs := make([]llm.Doc, 0, len(docs))
	for _, doc := range docs {
		llmDocs = append(llmDocs, llm.Doc{Title: doc.Title, Content: doc.Content, Context: doc.Context})
	}
	return y.backend.Rerank(query, llmDocs)
}

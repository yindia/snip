package rerank

import "snip/internal/util"

type Doc struct {
	Title   string
	Content string
	Context string
}

type Reranker interface {
	Rerank(query string, docs []Doc) ([]float64, error)
}

type LexicalReranker struct{}

func (l LexicalReranker) Rerank(query string, docs []Doc) ([]float64, error) {
	qTokens := util.Tokenize(query)
	qSet := map[string]struct{}{}
	for _, t := range qTokens {
		qSet[t] = struct{}{}
	}
	denom := float64(len(qSet))
	if denom == 0 {
		denom = 1
	}
	scores := make([]float64, len(docs))
	for i, doc := range docs {
		text := doc.Title + " " + doc.Context + " " + doc.Content
		tokens := util.Tokenize(text)
		hits := 0
		seen := map[string]struct{}{}
		for _, t := range tokens {
			if _, ok := qSet[t]; ok {
				if _, dup := seen[t]; !dup {
					hits++
					seen[t] = struct{}{}
				}
			}
		}
		scores[i] = float64(hits) / denom
	}
	return scores, nil
}

type FallbackReranker struct {
	Primary  Reranker
	Fallback Reranker
}

func (f FallbackReranker) Rerank(query string, docs []Doc) ([]float64, error) {
	if f.Primary == nil {
		return f.Fallback.Rerank(query, docs)
	}
	scores, err := f.Primary.Rerank(query, docs)
	if err != nil || len(scores) != len(docs) {
		return f.Fallback.Rerank(query, docs)
	}
	return scores, nil
}

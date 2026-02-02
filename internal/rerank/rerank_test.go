package rerank

import "testing"

func TestLexicalRerankerScores(t *testing.T) {
	rr := LexicalReranker{}
	docs := []Doc{
		{Title: "Foo", Content: "foo bar baz", Context: ""},
		{Title: "Foo", Content: "foo only", Context: ""},
	}
	scores, err := rr.Rerank("foo bar", docs)
	if err != nil {
		t.Fatalf("rerank error: %v", err)
	}
	if len(scores) != 2 {
		t.Fatalf("expected 2 scores, got %d", len(scores))
	}
	if scores[0] <= scores[1] {
		t.Fatalf("expected first doc to score higher: %v", scores)
	}
}

type errReranker struct{}

func (e errReranker) Rerank(_ string, _ []Doc) ([]float64, error) {
	return nil, errTest
}

type fixedReranker struct{}

func (f fixedReranker) Rerank(_ string, docs []Doc) ([]float64, error) {
	out := make([]float64, len(docs))
	for i := range out {
		out[i] = 0.5
	}
	return out, nil
}

var errTest = errorString("test error")

type errorString string

func (e errorString) Error() string { return string(e) }

func TestFallbackReranker(t *testing.T) {
	docs := []Doc{{Title: "a"}}
	fallback := fixedReranker{}
	rr := FallbackReranker{Primary: errReranker{}, Fallback: fallback}
	scores, err := rr.Rerank("q", docs)
	if err != nil {
		t.Fatalf("fallback error: %v", err)
	}
	if len(scores) != 1 || scores[0] != 0.5 {
		t.Fatalf("expected fallback scores, got %v", scores)
	}
}

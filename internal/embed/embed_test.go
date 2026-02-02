package embed

import "testing"

func TestHashEmbedderDeterministic(t *testing.T) {
	embedder := NewHashEmbedder(8)
	vecs1, err := embedder.Embed([]string{"hello world"})
	if err != nil {
		t.Fatalf("embed error: %v", err)
	}
	vecs2, err := embedder.Embed([]string{"hello world"})
	if err != nil {
		t.Fatalf("embed error: %v", err)
	}
	if len(vecs1) != 1 || len(vecs2) != 1 {
		t.Fatalf("expected one vector")
	}
	if len(vecs1[0]) != 8 {
		t.Fatalf("expected dimension 8, got %d", len(vecs1[0]))
	}
	for i := range vecs1[0] {
		if vecs1[0][i] != vecs2[0][i] {
			t.Fatalf("expected deterministic embedding")
		}
	}
}

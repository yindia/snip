package embed

import "testing"

func TestChunkText(t *testing.T) {
	words := make([]string, 1000)
	for i := range words {
		words[i] = "w"
	}
	text := ""
	for i, w := range words {
		if i > 0 {
			text += " "
		}
		text += w
	}
	chunks := ChunkText(text, 800, 120)
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}
	if chunks[0].Pos != 0 || chunks[1].Pos != 680 {
		t.Fatalf("unexpected positions: %d, %d", chunks[0].Pos, chunks[1].Pos)
	}
}

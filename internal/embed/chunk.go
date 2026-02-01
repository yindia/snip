package embed

import "strings"

type Chunk struct {
	Seq  int
	Pos  int
	Text string
}

// ChunkText splits text into token-based chunks with overlap.
func ChunkText(text string, size int, overlap int) []Chunk {
	if size <= 0 {
		return nil
	}
	if overlap < 0 {
		overlap = 0
	}
	step := size - overlap
	if step <= 0 {
		step = 1
	}
	tokens := strings.Fields(text)
	if len(tokens) == 0 {
		return nil
	}
	chunks := []Chunk{}
	seq := 0
	for i := 0; i < len(tokens); i += step {
		end := i + size
		if end > len(tokens) {
			end = len(tokens)
		}
		chunk := strings.Join(tokens[i:end], " ")
		chunks = append(chunks, Chunk{Seq: seq, Pos: i, Text: chunk})
		seq++
		if end == len(tokens) {
			break
		}
	}
	return chunks
}

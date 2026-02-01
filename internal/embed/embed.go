package embed

import (
	"encoding/binary"
	"hash/fnv"
	"math"

	"snip/internal/util"
)

type Embedder interface {
	Embed(texts []string) ([][]float32, error)
	Dim() int
	Name() string
}

type HashEmbedder struct {
	dim int
}

func NewHashEmbedder(dim int) *HashEmbedder {
	return &HashEmbedder{dim: dim}
}

func (h *HashEmbedder) Name() string {
	return "hash"
}

func (h *HashEmbedder) Dim() int {
	return h.dim
}

func (h *HashEmbedder) Embed(texts []string) ([][]float32, error) {
	out := make([][]float32, 0, len(texts))
	for _, text := range texts {
		vec := make([]float32, h.dim)
		tokens := util.Tokenize(text)
		for _, tok := range tokens {
			hash := fnv32(tok)
			idx := int(hash % uint32(h.dim))
			vec[idx] += 1.0
		}
		normalize(vec)
		out = append(out, vec)
	}
	return out, nil
}

func fnv32(s string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(s))
	return h.Sum32()
}

func normalize(vec []float32) {
	var sum float64
	for _, v := range vec {
		sum += float64(v * v)
	}
	if sum == 0 {
		return
	}
	denom := float32(math.Sqrt(sum))
	for i := range vec {
		vec[i] = vec[i] / denom
	}
}

func EncodeVector(vec []float32) []byte {
	buf := make([]byte, 4*len(vec))
	for i, v := range vec {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(v))
	}
	return buf
}

func DecodeVector(data []byte) []float32 {
	if len(data)%4 != 0 {
		return nil
	}
	count := len(data) / 4
	vec := make([]float32, count)
	for i := 0; i < count; i++ {
		bits := binary.LittleEndian.Uint32(data[i*4:])
		vec[i] = math.Float32frombits(bits)
	}
	return vec
}

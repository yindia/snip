//go:build yzma
// +build yzma

package llm

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/hybridgroup/yzma/pkg/llama"

	"snip/internal/config"
	"snip/internal/util"
)

type Backend struct {
	embedModel  llama.Model
	embedCtx    llama.Context
	embedVocab  llama.Vocab
	embedDim    int
	rankModel   llama.Model
	rankCtx     llama.Context
	rankVocab   llama.Vocab
	rankNClsOut int
	expandModel llama.Model
	expandVocab llama.Vocab

	embedMu  sync.Mutex
	rankMu   sync.Mutex
	expandMu sync.Mutex
}

var (
	llamaOnce    sync.Once
	llamaInitErr error
	llamaPath    string
)

func NewYzmaBackend(cfg config.Config) (*Backend, error) {
	if cfg.LlamaLibPath == "" {
		return nil, errors.New("llama_lib_path is not set")
	}
	if cfg.EmbedModel == "" || strings.EqualFold(cfg.EmbedModel, "hash") {
		return nil, errors.New("embed_model is not set")
	}
	if err := ensureLlama(cfg.LlamaLibPath, cfg.Debug); err != nil {
		return nil, err
	}
	if cfg.ModelCacheDir != "" {
		_ = os.MkdirAll(cfg.ModelCacheDir, 0o755)
	}

	embedPath, err := resolveModelPath(cfg.EmbedModel, cfg.ModelCacheDir)
	if err != nil {
		return nil, err
	}
	embedModel, embedCtx, embedVocab, embedDim, err := loadEmbedModel(embedPath)
	if err != nil {
		return nil, err
	}

	backend := &Backend{
		embedModel: embedModel,
		embedCtx:   embedCtx,
		embedVocab: embedVocab,
		embedDim:   embedDim,
	}

	if cfg.RerankModel != "" {
		if rankPath, err := resolveModelPath(cfg.RerankModel, cfg.ModelCacheDir); err == nil {
			if rankModel, rankCtx, rankVocab, nCls, err := loadRankModel(rankPath); err == nil {
				backend.rankModel = rankModel
				backend.rankCtx = rankCtx
				backend.rankVocab = rankVocab
				backend.rankNClsOut = nCls
			} else {
				util.Debugf("rerank model load failed: %v", err)
			}
		} else {
			util.Debugf("rerank model resolve failed: %v", err)
		}
	}

	if cfg.ExpandModel != "" {
		if expandPath, err := resolveModelPath(cfg.ExpandModel, cfg.ModelCacheDir); err == nil {
			if expandModel, expandVocab, err := loadExpandModel(expandPath); err == nil {
				backend.expandModel = expandModel
				backend.expandVocab = expandVocab
			} else {
				util.Debugf("expand model load failed: %v", err)
			}
		} else {
			util.Debugf("expand model resolve failed: %v", err)
		}
	}

	return backend, nil
}

func (b *Backend) Embed(texts []string) ([][]float32, error) {
	if b.embedCtx == 0 {
		return nil, errors.New("embed context not initialized")
	}
	b.embedMu.Lock()
	defer b.embedMu.Unlock()
	return embedTexts(b.embedCtx, b.embedVocab, b.embedDim, texts)
}

func (b *Backend) EmbedDim() int {
	return b.embedDim
}

func (b *Backend) CanRerank() bool {
	return b.rankModel != 0 && b.rankCtx != 0
}

func (b *Backend) CanExpand() bool {
	return b.expandModel != 0
}

func (b *Backend) FormatQuery(query string) string {
	return "task: search result | query: " + query
}

func (b *Backend) FormatDocument(title, text string) string {
	if strings.TrimSpace(title) == "" {
		title = "none"
	}
	return "title: " + title + " | text: " + text
}

func (b *Backend) Rerank(query string, docs []Doc) ([]float64, error) {
	if b.rankCtx == 0 || b.rankModel == 0 {
		return nil, errors.New("rerank model not available")
	}
	b.rankMu.Lock()
	defer b.rankMu.Unlock()
	mem, _ := llama.GetMemory(b.rankCtx)
	scores := make([]float64, 0, len(docs))
	for _, doc := range docs {
		prompt := formatRerankPrompt(query, doc)
		textScore, err := rankText(b.rankCtx, b.rankVocab, b.rankModel, b.rankNClsOut, prompt)
		if err != nil {
			return nil, err
		}
		scores = append(scores, textScore)
		if mem != 0 {
			_ = llama.MemoryClear(mem, true)
		}
	}
	return scores, nil
}

func (b *Backend) Expand(query string) ([]string, error) {
	if b.expandModel == 0 {
		return nil, errors.New("expand model not available")
	}
	b.expandMu.Lock()
	defer b.expandMu.Unlock()
	return expandQuery(b.expandModel, b.expandVocab, query)
}

type Doc struct {
	Title   string
	Content string
	Context string
}

func ensureLlama(path string, debug bool) error {
	llamaOnce.Do(func() {
		llamaPath = path
		if err := llama.Load(path); err != nil {
			llamaInitErr = err
			return
		}
		if !debug {
			llama.LogSet(llama.LogSilent())
		}
		llama.Init()
	})
	if llamaInitErr != nil {
		return llamaInitErr
	}
	if llamaPath != path {
		return fmt.Errorf("llama already initialized with %s", llamaPath)
	}
	return nil
}

func resolveModelPath(model string, cacheDir string) (string, error) {
	model = strings.TrimSpace(model)
	if model == "" {
		return "", errors.New("model path is empty")
	}
	if strings.HasPrefix(model, "hf:") {
		ref, err := parseHF(model)
		if err != nil {
			return "", err
		}
		filename := filepath.Base(ref.file)
		if filename == "" {
			return "", fmt.Errorf("invalid hf model file: %s", model)
		}
		if cacheDir == "" {
			return "", errors.New("model_cache_dir is not set")
		}
		path := filepath.Join(cacheDir, filename)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
		return "", fmt.Errorf("model not found: %s (download to %s)", model, path)
	}
	path := model
	if !filepath.IsAbs(path) {
		abs, err := filepath.Abs(path)
		if err == nil {
			path = abs
		}
	}
	if _, err := os.Stat(path); err != nil {
		return "", err
	}
	return path, nil
}

type hfRef struct {
	repo string
	file string
}

func parseHF(model string) (hfRef, error) {
	if !strings.HasPrefix(model, "hf:") {
		return hfRef{}, fmt.Errorf("not an hf uri: %s", model)
	}
	without := strings.TrimPrefix(model, "hf:")
	parts := strings.Split(without, "/")
	if len(parts) < 3 {
		return hfRef{}, fmt.Errorf("invalid hf uri: %s", model)
	}
	repo := strings.Join(parts[:2], "/")
	file := strings.Join(parts[2:], "/")
	return hfRef{repo: repo, file: file}, nil
}

func loadEmbedModel(path string) (llama.Model, llama.Context, llama.Vocab, int, error) {
	model, err := llama.ModelLoadFromFile(path, llama.ModelDefaultParams())
	if err != nil || model == 0 {
		return 0, 0, 0, 0, fmt.Errorf("embed model load failed: %w", err)
	}
	params := llama.ContextDefaultParams()
	params.Embeddings = 1
	params.PoolingType = llama.PoolingTypeMean
	ctx, err := llama.InitFromModel(model, params)
	if err != nil || ctx == 0 {
		llama.ModelFree(model)
		return 0, 0, 0, 0, fmt.Errorf("embed context init failed: %w", err)
	}
	vocab := llama.ModelGetVocab(model)
	dim := int(llama.ModelNEmbd(model))
	return model, ctx, vocab, dim, nil
}

func loadRankModel(path string) (llama.Model, llama.Context, llama.Vocab, int, error) {
	model, err := llama.ModelLoadFromFile(path, llama.ModelDefaultParams())
	if err != nil || model == 0 {
		return 0, 0, 0, 0, fmt.Errorf("rerank model load failed: %w", err)
	}
	params := llama.ContextDefaultParams()
	params.Embeddings = 1
	params.PoolingType = llama.PoolingTypeRank
	ctx, err := llama.InitFromModel(model, params)
	if err != nil || ctx == 0 {
		llama.ModelFree(model)
		return 0, 0, 0, 0, fmt.Errorf("rerank context init failed: %w", err)
	}
	vocab := llama.ModelGetVocab(model)
	nCls := int(llama.ModelNClsOut(model))
	if nCls == 0 {
		nCls = 1
	}
	return model, ctx, vocab, nCls, nil
}

func loadExpandModel(path string) (llama.Model, llama.Vocab, error) {
	model, err := llama.ModelLoadFromFile(path, llama.ModelDefaultParams())
	if err != nil || model == 0 {
		return 0, 0, fmt.Errorf("expand model load failed: %w", err)
	}
	vocab := llama.ModelGetVocab(model)
	return model, vocab, nil
}

func limitTokens(ctx llama.Context, tokens []llama.Token) []llama.Token {
	max := int(llama.NUBatch(ctx))
	if max <= 0 {
		max = int(llama.NBatch(ctx))
	}
	if max <= 0 || len(tokens) <= max {
		return tokens
	}
	return tokens[:max]
}

func embedTexts(ctx llama.Context, vocab llama.Vocab, dim int, texts []string) ([][]float32, error) {
	if ctx == 0 {
		return nil, errors.New("invalid embed context")
	}
	mem, _ := llama.GetMemory(ctx)
	out := make([][]float32, 0, len(texts))
	for _, text := range texts {
		tokens := llama.Tokenize(vocab, text, true, true)
		tokens = limitTokens(ctx, tokens)
		if len(tokens) == 0 {
			out = append(out, make([]float32, dim))
			continue
		}
		batch := llama.BatchGetOne(tokens)
		llama.Encode(ctx, batch)
		emb, err := llama.GetEmbeddingsSeq(ctx, 0, int32(dim))
		if err != nil {
			return nil, err
		}
		if emb == nil {
			return nil, errors.New("embedding returned nil")
		}
		vec := append([]float32(nil), emb...)
		out = append(out, vec)
		if mem != 0 {
			_ = llama.MemoryClear(mem, true)
		}
	}
	return out, nil
}

func rankText(ctx llama.Context, vocab llama.Vocab, model llama.Model, nCls int, text string) (float64, error) {
	tokens := llama.Tokenize(vocab, text, true, true)
	tokens = limitTokens(ctx, tokens)
	if len(tokens) == 0 {
		return 0, nil
	}
	batch := llama.BatchGetOne(tokens)
	llama.Encode(ctx, batch)
	vals, err := llama.GetEmbeddingsSeq(ctx, 0, int32(nCls))
	if err != nil {
		return 0, err
	}
	if len(vals) == 0 {
		return 0, nil
	}
	if len(vals) == 1 {
		return float64(vals[0]), nil
	}
	labelScore := pickRerankScore(model, vals)
	return float64(labelScore), nil
}

func pickRerankScore(model llama.Model, scores []float32) float32 {
	for i := range scores {
		label := strings.ToLower(llama.ModelClsLabel(model, uint32(i)))
		if strings.Contains(label, "yes") || strings.Contains(label, "true") || strings.Contains(label, "relevant") {
			return scores[i]
		}
	}
	return scores[len(scores)-1]
}

func formatRerankPrompt(query string, doc Doc) string {
	builder := strings.Builder{}
	builder.WriteString("query: ")
	builder.WriteString(strings.TrimSpace(query))
	builder.WriteString("\n")
	if strings.TrimSpace(doc.Title) != "" {
		builder.WriteString("title: ")
		builder.WriteString(strings.TrimSpace(doc.Title))
		builder.WriteString("\n")
	}
	if strings.TrimSpace(doc.Context) != "" {
		builder.WriteString("context: ")
		builder.WriteString(strings.TrimSpace(doc.Context))
		builder.WriteString("\n")
	}
	builder.WriteString("text: ")
	builder.WriteString(strings.TrimSpace(doc.Content))
	return builder.String()
}

func expandQuery(model llama.Model, vocab llama.Vocab, query string) ([]string, error) {
	params := llama.ContextDefaultParams()
	ctx, err := llama.InitFromModel(model, params)
	if err != nil || ctx == 0 {
		return nil, fmt.Errorf("expand context init failed: %w", err)
	}
	defer llama.Free(ctx)

	prompt := "Generate 2 alternative search queries for: " + query + "\nReturn one per line."
	tokens := llama.Tokenize(vocab, prompt, true, true)
	tokens = limitTokens(ctx, tokens)
	if len(tokens) == 0 {
		return nil, nil
	}
	batch := llama.BatchGetOne(tokens)

	sampler := llama.SamplerChainInit(llama.SamplerChainDefaultParams())
	llama.SamplerChainAdd(sampler, llama.SamplerInitGreedy())
	defer llama.SamplerFree(sampler)

	var out strings.Builder
	for pos := int32(0); pos < 128; pos += batch.NTokens {
		llama.Decode(ctx, batch)
		token := llama.SamplerSample(sampler, ctx, -1)
		if llama.VocabIsEOG(vocab, token) {
			break
		}
		buf := make([]byte, 256)
		l := llama.TokenToPiece(vocab, token, buf, 0, false)
		if l > 0 {
			out.Write(buf[:l])
		}
		batch = llama.BatchGetOne([]llama.Token{token})
	}

	lines := strings.Split(out.String(), "\n")
	alts := make([]string, 0, 2)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		alts = append(alts, line)
		if len(alts) >= 2 {
			break
		}
	}
	return alts, nil
}

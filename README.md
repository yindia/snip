# 🔎 SNIP - Search, Navigate, Index, Parse

An on-device search engine for everything you need to remember. Index your Markdown notes, meeting transcripts, documentation, and knowledge bases. Search with keywords or natural language. Ideal for agentic workflows.

SNIP combines BM25 full-text search, deterministic offline vector search, and a hybrid reranking pipeline. Single binary, fully offline by default.

## ✨ Highlights

- ⚡ Fast local search with BM25 + vectors
- 🔒 Fully offline by default
- 🧠 Hybrid fusion with reranking
- 📦 Single binary (macOS/Linux/Windows)

## 🚀 Installation

Homebrew:

```sh
brew install yindia/homebrew-yindia/snip
```

Install script:

```sh
curl -fsSL https://raw.githubusercontent.com/yindia/snip/refs/heads/main/install.sh | sh
```

Manual build:

```sh
go build ./cmd/snip
```

## ⚡ Quick Start

```sh
# create collections for notes, meetings, and docs
snip collection add ~/notes --name notes
snip collection add ~/Documents/meetings --name meetings
snip collection add ~/work/docs --name docs

# add context to improve results
snip context add snip://notes "Personal notes and ideas"
snip context add snip://meetings "Meeting transcripts and notes"
snip context add snip://docs "Work documentation"

# index content
snip update

# generate embeddings for semantic search (offline)
snip embed

# search across everything
snip search "project timeline"           # fast keyword search
snip vsearch "how to deploy"             # semantic search
snip query "quarterly planning process"  # hybrid + reranking (best quality)

# get a specific document
snip get "meetings/2024-01-15.md"

# get a document by docid (shown in search results)
snip get "#abc123def4567890"

# get multiple documents by glob pattern
snip get "journals/2025-05*.md"

# search within a specific collection
snip search "API" -c notes

# export all matches for an agent
snip search "API" --all --files --min-score 0.3
```

### 🤖 Using With AI Agents

SNIP's `--json` and `--files` output formats are designed for agentic workflows:

```sh
# structured results
snip search "authentication" --json -n 10

# list all relevant files above a threshold
snip query "error handling" --all --files --min-score 0.4

# retrieve full document content
snip get "docs/api-reference.md"
```

Example pipeline with a local LLM (Ollama):

```sh
# search, then summarize with a local model
snip query "payment retries" --json -n 8 | \
  ollama run mistral "Summarize key findings and cite docids."
```

### 🧩 MCP Server

SNIP can run an MCP server over STDIO for agent integrations:

```sh
snip mcp
```

By default, `snip mcp` uses the same index directory as your last CLI run.
Override with `--index` or `SNIP_INDEX_DIR` if you keep multiple indexes.

**Tools exposed:**
- `snip_search` - 🔍 Fast BM25 keyword search
- `snip_vsearch` - 🧠 Semantic vector search (requires embeddings)
- `snip_query` - 🧬 Hybrid search with fusion + reranking
- `snip_get` - 📄 Retrieve one or many documents (path/docid, glob, or list)
- `snip_status` - 📊 Index stats and collection info

**Resources:**
- `snip://{collection}/{relative/path}` - returns document text with line numbers

**Claude Desktop configuration** (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "snip": {
      "command": "snip",
      "args": ["mcp"]
    }
  }
}
```

**Claude Code configuration** (`~/.claude/settings.json`):

```json
{
  "mcpServers": {
    "snip": {
      "command": "snip",
      "args": ["mcp"]
    }
  }
}
```

## 🧱 Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         SNIP Hybrid Search Pipeline                         │
└─────────────────────────────────────────────────────────────────────────────┘

                         ┌─────────────────────────┐
                         │       User Query        │
                         └────────────┬────────────┘
                                      │
                   ┌──────────────────┴──────────────────┐
                   ▼                                     ▼
        ┌────────────────────────┐            ┌────────────────────────┐
        │ Query Expansion        │            │ Original Query (×2)     │
        │ deterministic variants │            └────────────┬───────────┘
        └────────────┬───────────┘                         │
                     │ 1-2 alternatives                   │
                     └────────────┬────────────────────────┘
                                  ▼
                       ┌────────────────────┐
                       │ Query Set (N<=3)   │
                       └───────────┬────────┘
                                   │
        ┌──────────────────────────┼──────────────────────────┐
        ▼                          ▼                          ▼
  ┌───────────────┐         ┌───────────────┐          ┌───────────────┐
  │ BM25 (FTS5)   │         │ Vector Search │          │ BM25 (FTS5)   │
  └───────┬───────┘         └───────┬───────┘          └───────┬───────┘
          │                         │                          │
          └──────────────┬──────────┴──────────┬───────────────┘
                         ▼                     ▼
                 ┌────────────────────────────────────┐
                 │   RRF Fusion + Top-Rank Bonus      │
                 │   original query weighted ×2       │
                 │   keep top 30 candidates           │
                 └──────────────────┬─────────────────┘
                                    │
                                    ▼
                         ┌───────────────────────┐
                         │  Local Re-ranking     │
                         │ (yzma or overlap)     │
                         └───────────┬───────────┘
                                     │
                                     ▼
                         ┌───────────────────────┐
                         │  Position-Aware Blend │
                         │  Top 1-3:  75% RRF    │
                         │  Top 4-10: 60% RRF    │
                         │  Top 11+:  40% RRF    │
                         └───────────────────────┘
```

## 📊 Score Normalization & Fusion

### Search Backends

| Backend | Raw Score | Normalization | Range |
|---------|-----------|---------------|-------|
| **FTS (BM25)** | SQLite FTS5 BM25 | Min-max within result set | 0.0 to 1.0 |
| **Vector** | Cosine similarity | Min-max within result set | 0.0 to 1.0 |
| **Reranker** | Yzma-based LLM or lexical overlap | Min-max within result set | 0.0 to 1.0 |

### Fusion Strategy

The `query` command uses **Reciprocal Rank Fusion (RRF)** with position-aware blending:

1. **Query Expansion**: Original query (×2 weighting) + 1–2 deterministic variants
2. **Parallel Retrieval**: Each query searches both FTS and vector indexes
3. **RRF Fusion**: Combine all result lists using `score = Σ(1/(k+rank+1))` where k=60
4. **Top-Rank Bonus**: Documents ranking #1 in any list get +0.05, #2-3 get +0.02
5. **Top-K Selection**: Take top 30 candidates for reranking
6. **Re-ranking**: Yzma-based LLM reranker, fallback to lexical overlap
7. **Position-Aware Blending**:
   - RRF rank 1-3: 75% retrieval, 25% reranker
   - RRF rank 4-10: 60% retrieval, 40% reranker
   - RRF rank 11+: 40% retrieval, 60% reranker

## ✅ Requirements

- Go 1.24+ toolchain for building from source
- SQLite is embedded via Go (no external dependency)
- Optional: llama.cpp shared libraries for yzma-based models

## 📚 Usage

### 📁 Collection Management

```sh
# create a collection from current directory
snip collection add . --name myproject

# create a collection with explicit path and custom file extensions
# extensions match files recursively in any subdirectory
snip collection add ~/repo --name repo --extension go --extension py

# match all markdown files recursively
snip collection add ~/repo --name repo --extension md

# indexing respects .gitignore files under the collection root

# list all collections
snip collection list

# remove a collection
snip collection remove myproject

# rename a collection
snip collection rename myproject my-project

# list files in a collection
snip ls notes
snip ls notes/subfolder
```

### 🧠 Generate Vector Embeddings

```sh
# embed all indexed documents (800 tokens/chunk, 15% overlap)
snip embed

# force re-embed everything
snip embed -f
```

### 🧭 Context Management

Context adds descriptive metadata to collections and paths, helping search understand your content.

```sh
# add context to a collection (using snip:// virtual paths)
snip context add snip://notes "Personal notes and ideas"
snip context add snip://docs/api "API documentation"

# add context using an absolute path
snip context add ~/notes/work "Work-related notes"

# list all contexts
snip context list

# remove context
snip context rm snip://notes/old
```

### 🔍 Search Commands

```
┌──────────────────────────────────────────────────────────────────┐
│                        Search Modes                              │
├──────────┬───────────────────────────────────────────────────────┤
│ search   │ BM25 full-text search only                           │
│ vsearch  │ Vector semantic search only                          │
│ query    │ Hybrid: FTS + Vector + Expansion + Reranking          │
└──────────┴───────────────────────────────────────────────────────┘
```

```sh
# full-text search (fast, keyword-based)
snip search "authentication flow"

# vector search (semantic similarity)
snip vsearch "how to login"

# hybrid search with reranking (best quality)
snip query "user authentication"
```

### ⚙️ Options

```sh
# search options
-n <num>           # number of results (default: 5)
-c, --collection   # restrict search to a specific collection
--all              # search all collections
--min-score <num>  # minimum score threshold (0.0–1.0)
--full             # include full document content
--line-numbers     # add line numbers to output

# output formats
--files            # output file paths only
--json             # JSON output
--csv              # CSV output
--md               # Markdown output
--xml              # XML output
```

### 🖨️ Output Format (Human)

```
Software Craftsmanship  (score: 0.9300)
docs/guide.md  [a1b2c3]
context: Work documentation

This section covers the craftsmanship of building
quality software with attention to detail.
```

## 💾 Data Storage

Index stored at: `~/.cache/snip/index.sqlite` (respects `XDG_CACHE_HOME`).

Schema:

```sql
collections     -- Indexed directories with name and extensions
path_contexts   -- Context descriptions by virtual path (snip://...)
documents       -- Markdown content with metadata and docid (16-char hash)
documents_fts   -- FTS5 full-text index
content_vectors -- Embedding chunks (hash, seq, pos, 800 tokens each)
```

## 🧰 How It Works

### 🧱 Indexing Flow

```
Collection ──► Extensions ──► Markdown Files ──► Parse Title ──► Hash Content
    │                                                   │              │
    │                                                   │              ▼
    │                                                   │         Generate docid
    │                                                   │         (16-char hash of collection+path)
    │                                                   │              │
    └──────────────────────────────────────────────────►└──► Store in SQLite
                                                                       │
                                                                       ▼
                                                                  FTS5 Index
```

### 🧠 Embedding Flow

Documents are chunked into 800-token pieces with 15% overlap:

```
Document ──► Chunk (800 tokens) ──► Hash Embedder ──► Store Vectors
                │
                └─► Chunks stored with:
                    - hash: document hash
                    - seq: chunk sequence (0, 1, 2...)
                    - pos: token position in original
```

### 🧬 Query Flow (Hybrid)

```
Query ──► Expansion ──► [Original, Variant 1, Variant 2]
                │
      ┌─────────┴─────────┐
      ▼                   ▼
   For each query:     FTS (BM25)
      │                   │
      ▼                   ▼
   Vector Search      Ranked List
      │
      ▼
   Ranked List
      │
      └─────────┬─────────┘
                ▼
         RRF Fusion (k=60)
         Original query ×2 weight
         Top-rank bonus: +0.05/#1, +0.02/#2-3
                │
                ▼
         Top 30 candidates
                │
                ▼
         Local Re-ranking (yzma or overlap)
                │
                ▼
         Position-Aware Blend
         Rank 1-3:  75% RRF / 25% reranker
         Rank 4-10: 60% RRF / 40% reranker
         Rank 11+:  40% RRF / 60% reranker
                │
                ▼
         Final Results
```

## 🔧 Configuration

SNIP reads YAML or JSON config (if present) and environment variables. Flags always override config.

Default config path (if present):

- `~/.config/snip/config.yaml`

Config keys:

- `index_dir`
- `embed_model`
- `rerank_model`
- `expand_model`
- `model_cache_dir`
- `llama_lib_path`
- `model` (legacy alias for `embed_model`)
- `debug`
- `no_color`

Env vars:

- `SNIP_INDEX_DIR`
- `SNIP_EMBED_MODEL`
- `SNIP_RERANK_MODEL`
- `SNIP_EXPAND_MODEL`
- `SNIP_MODEL_CACHE_DIR`
- `SNIP_LLAMA_LIB`
- `SNIP_MODEL` (legacy alias for `SNIP_EMBED_MODEL`)
- `SNIP_DEBUG`

### 🧠 Model Configuration (Yzma)

SNIP supports local GGUF models via yzma (llama.cpp without CGO). The flow is:

1) Build SNIP with yzma support.
2) Install llama.cpp shared libraries.
3) Download GGUF models into a cache directorysnip.
4) Point SNIP at the libraries + models using config or env vars.

Build with the `yzma` tag:

```sh
go build -tags yzma ./main.go
```

Install llama.cpp libs and download models:

```sh
yzma install --lib ~/.cache/snip/llama
yzma model get -u https://huggingface.co/ggml-org/embeddinggemma-300M-GGUF/resolve/main/embeddinggemma-300M-Q8_0.gguf -o ~/.cache/snip/models
yzma model get -u https://huggingface.co/ggml-org/Qwen3-Reranker-0.6B-Q8_0-GGUF/resolve/main/qwen3-reranker-0.6b-q8_0.gguf -o ~/.cache/snip/models
yzma model get -u https://huggingface.co/tobil/qmd-query-expansion-1.7B-gguf/resolve/main/qmd-query-expansion-1.7B-q4_k_m.gguf -o ~/.cache/snip/models
```

Configure SNIP (config file or env vars). Example `~/.config/snip/config.yaml`:

```yaml
llama_lib_path: ~/.cache/snip/llama
model_cache_dir: ~/.cache/snip/models
embed_model: hf:ggml-org/embeddinggemma-300M-GGUF/embeddinggemma-300M-Q8_0.gguf
rerank_model: hf:ggml-org/Qwen3-Reranker-0.6B-Q8_0-GGUF/qwen3-reranker-0.6b-q8_0.gguf
expand_model: hf:tobil/qmd-query-expansion-1.7B-gguf/qmd-query-expansion-1.7B-q4_k_m.gguf
```

Or set env vars instead:

```sh
export SNIP_LLAMA_LIB=~/.cache/snip/llama
export SNIP_MODEL_CACHE_DIR=~/.cache/snip/models
export SNIP_EMBED_MODEL=hf:ggml-org/embeddinggemma-300M-GGUF/embeddinggemma-300M-Q8_0.gguf
export SNIP_RERANK_MODEL=hf:ggml-org/Qwen3-Reranker-0.6B-Q8_0-GGUF/qwen3-reranker-0.6b-q8_0.gguf
export SNIP_EXPAND_MODEL=hf:tobil/qmd-query-expansion-1.7B-gguf/qmd-query-expansion-1.7B-q4_k_m.gguf
```

If models or the llama.cpp libraries are missing, SNIP falls back to hash embeddings and lexical reranking. To force pure-Go mode, set `embed_model: hash` (or `SNIP_MODEL=hash`).

## 🙏 Credits

SNIP is heavily inspired by https://github.com/tobi/qmd/tree/main.

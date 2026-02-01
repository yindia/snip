# SNIP - Search, Navigate, Index, Parse

An on-device search engine for everything you need to remember. Index your Markdown notes, meeting transcripts, documentation, and knowledge bases. Search with keywords or natural language. Ideal for agentic workflows.

SNIP combines BM25 full-text search, deterministic offline vector search, and a hybrid reranking pipeline. Single binary, fully offline by default.

## Installation

Homebrew (tap placeholder):

```sh
brew install snip
```

Install script (placeholder):

```sh
curl -fsSL https://example.com/snip/install.sh | sh
```

Manual build:

```sh
go build ./cmd/snip
```

## Quick Start

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
snip get "#abc123"

# get multiple documents by glob pattern
snip multi-get "journals/2025-05*.md"

# search within a specific collection
snip search "API" -c notes

# export all matches for an agent
snip search "API" --all --files --min-score 0.3
```

### Using With AI Agents

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

### MCP Server (Planned)

SNIP is CLI-only in v1. MCP integration is planned but not available yet.

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         SNIP Hybrid Search Pipeline                         │
└─────────────────────────────────────────────────────────────────────────────┘

                              ┌─────────────────┐
                              │   User Query    │
                              └────────┬────────┘
                                       │
                        ┌──────────────┴──────────────┐
                        ▼                             ▼
               ┌────────────────┐            ┌────────────────┐
               │ Query Expansion│            │  Original Query│
               │ (deterministic)│            │   (×2 weight)  │
               └───────┬────────┘            └───────┬────────┘
                       │                             │
                       │ 1–2 alternative queries     │
                       └──────────────┬──────────────┘
                                      │
              ┌───────────────────────┼───────────────────────┐
              ▼                       ▼                       ▼
     ┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
     │ Original Query  │     │ Expanded Query 1│     │ Expanded Query 2│
     └────────┬────────┘     └────────┬────────┘     └────────┬────────┘
              │                       │                       │
      ┌───────┴───────┐       ┌───────┴───────┐       ┌───────┴───────┐
      ▼               ▼       ▼               ▼       ▼               ▼
  ┌───────┐       ┌───────┐ ┌───────┐     ┌───────┐ ┌───────┐     ┌───────┐
  │ BM25  │       │Vector │ │ BM25  │     │Vector │ │ BM25  │     │Vector │
  │(FTS5) │       │Search │ │(FTS5) │     │Search │ │(FTS5) │     │Search │
  └───┬───┘       └───┬───┘ └───┬───┘     └───┬───┘ └───┬───┘     └───┬───┘
      │               │         │             │         │             │
      └───────┬───────┘         └──────┬──────┘         └──────┬──────┘
              │                        │                       │
              └────────────────────────┼───────────────────────┘
                                       │
                                       ▼
                          ┌───────────────────────┐
                          │   RRF Fusion + Bonus  │
                          │  Original query: ×2   │
                          │  Top-rank bonus: +0.05│
                          │     Top 30 Kept       │
                          └───────────┬───────────┘
                                      │
                                      ▼
                          ┌───────────────────────┐
                          │  Lexical Re-ranking   │
                          │ (overlap fallback)    │
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

## Score Normalization & Fusion

### Search Backends

| Backend | Raw Score | Notes | Range |
|---------|-----------|-------|-------|
| **FTS (BM25)** | SQLite FTS5 BM25 | SNIP uses `-bm25()` so higher is better | 0 to ~25+ |
| **Vector** | Cosine similarity | Deterministic hash embedder in v1 | 0.0 to 1.0 |
| **Reranker** | Lexical overlap | Token overlap between query and doc | 0.0 to 1.0 |

### Fusion Strategy

The `query` command uses **Reciprocal Rank Fusion (RRF)** with position-aware blending:

1. **Query Expansion**: Original query (×2 weighting) + 1–2 deterministic variants
2. **Parallel Retrieval**: Each query searches both FTS and vector indexes
3. **RRF Fusion**: Combine all result lists using `score = Σ(1/(k+rank+1))` where k=60
4. **Top-Rank Bonus**: Documents ranking #1 in any list get +0.05, #2-3 get +0.02
5. **Top-K Selection**: Take top 30 candidates for reranking
6. **Re-ranking**: Lexical overlap fallback (local reranker interface ready)
7. **Position-Aware Blending**:
   - RRF rank 1-3: 75% retrieval, 25% reranker
   - RRF rank 4-10: 60% retrieval, 40% reranker
   - RRF rank 11+: 40% retrieval, 60% reranker

## Requirements

- Go toolchain for building from source
- SQLite is embedded via Go (no external dependency)

## Usage

### Collection Management

```sh
# create a collection from current directory
snip collection add . --name myproject

# create a collection with explicit path and custom glob masks
snip collection add ~/repo --name repo --mask "*.go" --mask "*.py"

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

### Generate Vector Embeddings

```sh
# embed all indexed documents (800 tokens/chunk, 15% overlap)
snip embed

# force re-embed everything
snip embed -f
```

### Context Management

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

### Search Commands

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

### Options

```sh
# search options
-n <num>           # number of results (default: 5)
-c, --collection   # restrict search to a specific collection
--all              # search all collections
--min-score <num>  # minimum score threshold (default: 0)
--full             # include full document content
--line-numbers     # add line numbers to output

# output formats
--files            # output file paths only
--json             # JSON output
--csv              # CSV output
--md               # Markdown output
--xml              # XML output
```

### Output Format (Human)

```
Software Craftsmanship  (score: 0.9300)
docs/guide.md  [a1b2c3]
context: Work documentation

This section covers the craftsmanship of building
quality software with attention to detail.
```

## Data Storage

Index stored at: `~/.cache/snip/index.sqlite` (respects `XDG_CACHE_HOME`).

Schema:

```sql
collections     -- Indexed directories with name and glob patterns
path_contexts   -- Context descriptions by virtual path (snip://...)
documents       -- Markdown content with metadata and docid (6-char hash)
documents_fts   -- FTS5 full-text index
content_vectors -- Embedding chunks (hash, seq, pos, 800 tokens each)
```

## How It Works

### Indexing Flow

```
Collection ──► Glob Pattern ──► Markdown Files ──► Parse Title ──► Hash Content
    │                                                   │              │
    │                                                   │              ▼
    │                                                   │         Generate docid
    │                                                   │         (6-char hash)
    │                                                   │              │
    └──────────────────────────────────────────────────►└──► Store in SQLite
                                                                       │
                                                                       ▼
                                                                  FTS5 Index
```

### Embedding Flow

Documents are chunked into 800-token pieces with 15% overlap:

```
Document ──► Chunk (800 tokens) ──► Hash Embedder ──► Store Vectors
                │
                └─► Chunks stored with:
                    - hash: document hash
                    - seq: chunk sequence (0, 1, 2...)
                    - pos: token position in original
```

### Query Flow (Hybrid)

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
         Lexical Re-ranking
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

## Configuration

SNIP reads YAML or JSON config (if present) and environment variables. Flags always override config.

Default config path (if present):

- `~/.config/snip/config.yaml`

Config keys:

- `index_dir`
- `model`
- `debug`
- `no_color`

Env vars:

- `SNIP_INDEX_DIR`
- `SNIP_MODEL`
- `SNIP_DEBUG`

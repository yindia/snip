# SNIP - Search, Navigate, Index, Parse

SNIP is a local-first CLI search engine for Markdown notes, meeting transcripts, and technical documentation. It indexes Markdown into SQLite (FTS5 BM25), supports deterministic offline embeddings, and offers a hybrid query pipeline. Single binary, fully offline by default.

## Installation

Homebrew (tap placeholder):

```sh
brew tap yindia/snip
brew install snip
```
Install script:

```sh
curl -fsSL https://raw.githubusercontent.com/yindia/snip/refs/heads/main/install.sh | sh
```

Manual build:

```sh
go build ./cmd/snip
```

## Quick Start

```sh
# build

go build ./cmd/snip

# create collections
./snip collection add ~/notes --name notes
./snip collection add ~/Documents/meetings --name meetings
./snip collection add ~/work/docs --name docs

# add context to improve results
./snip context add snip://notes "Personal notes and ideas"
./snip context add snip://meetings "Meeting transcripts and notes"
./snip context add snip://docs "Work documentation"

# index content
./snip update

# generate embeddings (offline)
./snip embed

# search
./snip search "project timeline"           # BM25 keyword search
./snip vsearch "how to deploy"             # Vector search
./snip query "quarterly planning process"  # Hybrid (best quality)

# get a document
./snip get "meetings/2024-01-15.md"

# get a document by docid (shown in search results)
./snip get "#abc123"

# get multiple documents by glob
./snip multi-get "journals/2025-05*.md"

# limit to a collection
./snip search "API" -c notes

# export for agents
./snip search "API" --all --files --min-score 0.3
```

## Using With AI Agents

SNIP supports agent-friendly outputs:

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
# search, then summarize the results with a local model
snip query "payment retries" --json -n 8 | \
  ollama run mistral "Summarize key findings and cite docids."
```

## Example: Searching Code

By default SNIP indexes Markdown. To index raw code files, add a collection with a mask and re-index:

```sh
# index Go + Python source files (repeatable masks)
snip collection add ~/work/myrepo --name myrepo --mask "*.go" --mask "*.py"
snip update

# keyword search for identifiers or functions
snip search "main" -c myrepo
snip search "ServeHTTP" -c myrepo

# hybrid search for mixed natural language + code
snip query "parse config file" -c myrepo
```

## Commands

### Collections

- `snip collection add <path> --name <name> [--mask "<glob>"]...`
- `snip collection list`
- `snip collection remove <name>`
- `snip collection rename <old> <new>`
- `snip ls <collection> [<subpath>]`

### Contexts

- `snip context add <path_or_snip_virtual> "<description>"`
- `snip context list`
- `snip context rm <path_or_snip_virtual>`

Virtual paths:

- `snip://<collection>/<optional_subpath>`

### Indexing

- `snip update`
- `snip status`
- `snip cleanup`

### Embeddings

- `snip embed [-f]`

### Search

- `snip search "<query>"` (FTS only)
- `snip vsearch "<query>"` (vector only)
- `snip query "<query>"` (hybrid)

Common flags:

- `-n <int>` (default 5)
- `-c, --collection <name>`
- `--all`
- `--min-score <float>`
- `--full`
- `--line-numbers`
- Output: `--files`, `--json`, `--csv`, `--md`, `--xml`

### Retrieval

- `snip get "<collection_relative_path>"`
- `snip get "#<docid>"`
- `snip get "<path>:<line>"`
- `snip multi-get "<glob>" | "a.md,b.md,#abc123"`

## Output Formats

- Default: colorized human output
- Files only: `--files`
- Structured: `--json`, `--csv`, `--md`, `--xml`

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

## Data Storage

Index stored at: `~/.cache/snip/index.sqlite` (respects `XDG_CACHE_HOME`).

Schema:

- `collections`: indexed roots and masks
- `path_contexts`: context descriptions by virtual path
- `documents`: content + metadata + hash/docid
- `documents_fts`: FTS5 full-text index
- `content_vectors`: chunk embeddings by content hash

## Embeddings

- Chunk size: ~800 tokens
- Overlap: ~15% (~120 tokens)
- Default model: deterministic `hash` embedder (offline)
- Model cache: `~/.cache/snip/models`

Models are behind interfaces so you can plug in a real local model later.

## Hybrid Query Pipeline

1. **Query expansion**: deterministic offline alternatives (1–2).
2. **Parallel retrieval**: BM25 + vector similarity.
3. **Fusion**: RRF (k=60), original query weighted x2, top-rank bonus.
4. **Reranking**: interface for local reranker, default lexical overlap fallback.
5. **Blending**: weighted mix of retrieval + rerank by rank tier.

## Troubleshooting

- **No results after `collection add`**: run `snip update` to index files.
- **FTS5 syntax errors**: queries with special characters are sanitized and retried.
- **Large corpora**: use `--collection` to scope queries and keep things fast.

## Build

```sh
go build ./cmd/snip
```

## Tests

```sh
go test ./...
```

# Product Search Relevance with RedisVL for Golang

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go 1.25+](https://img.shields.io/badge/Go-1.25%2B-blue.svg)](https://go.dev/dl/)
[![Redis Query Engine](https://img.shields.io/badge/Redis-Query%20Engine-DC382D.svg)](https://redis.io/docs/latest/develop/interact/search-and-query/)
[![RedisVL for Golang](https://img.shields.io/badge/RedisVL-Golang-DC382D.svg)](https://github.com/redis-developer/redis-vl-golang)

## 🌟 Overview

Welcome to this hands-on workshop where you'll build and evaluate ecommerce
product search with Go and Redis. You start with a working product-search
service — the HTTP API skeleton and the web UI are provided complete — and an
empty Redis database. Lab by lab, you implement the search internals with
[RedisVL for Golang](https://github.com/redis-developer/redis-vl-golang):
embeddings, indexing, vector search, filters, hybrid search, and facets. Then
you stop guessing and start measuring: using the
[WANDS](https://github.com/wayfair/WANDS) relevance judgments and the
[Redis Retrieval Optimizer](https://github.com/redis-applied-ai/redis-retrieval-optimizer),
you score every configuration and leave with an evidence-backed recommendation
for production.

### 🤔 Why This Workshop?

Search quality drops when you disregard design principles like:

- Full-text search alone can miss intent when wording changes even minimally
- Vector search alone can return semantically related but lexically weak matches
- Tuning by eyeballing results does not survive contact with real users

This workshop teaches the full loop: build the retrieval, observe its behavior,
**measure it with graded relevance judgments**, and choose the next experiment.

### 🎯 What You'll Build

By the end of this workshop, you'll have a complete Redis-powered search app with:

- A deterministic, judged WANDS product-search sample (600 products, 24 queries)
- Local in-process embeddings (ONNX, no API keys) behind a Redis embeddings cache
- A RedisVL index with text, tag, numeric, and vector fields
- Vector, filtered vector, hybrid (`FT.HYBRID`), and faceted search — visible
  live in the UI, switchable per query
- A relevance loop: nDCG@10 and Recall@25 against real human judgments
- A Retrieval Optimizer study across index types (FLAT / HNSW / SVS-VAMANA),
  embedding models, and query strategies — one ranked table, one recommendation

## 📋 Prerequisites

### Required knowledge

- Basic Go familiarity
- Basic understanding of search concepts
- Familiarity with command-line tools
- Basic understanding of Docker and Git

### Required software

#### Option 1: GitHub Codespaces

- GitHub account with Codespaces access
- Browser or VS Code with Codespaces support

#### Option 2: Dev Containers locally

- [Docker](https://docs.docker.com/get-docker/)
- An IDE compatible with [Dev Containers](https://containers.dev/)
  (VS Code, GoLand/IntelliJ)

#### Option 3: Local development

- [Go 1.25+](https://go.dev/dl/)
- [Docker](https://docs.docker.com/get-docker/)
- ONNX Runtime (`brew install onnxruntime` on macOS; `apt install
  libonnxruntime` or the [official releases](https://onnxruntime.ai/docs/install/) on Linux)
- [uv](https://docs.astral.sh/uv/) (only for Lab 8)

Check your setup at any time:

```bash
make doctor
```

### Required accounts

No paid account is required. Everything runs locally — embeddings included.
(OpenAI embeddings appear only as an optional tuning comparison for those who
bring a key.)

## 🗺️ Workshop Structure

This workshop has an estimated duration of 90–120 minutes, organized into
progressive labs. Each lab ends with the running service able to do something
it could not do before — watch the UI come alive as you go.

| Lab | Topic | Duration |
| --- | ----- | -------- |
| 0 | Setup: data, Redis, and the empty shelf | 10 min |
| 1 | Local embeddings with a Redis cache | 15 min |
| 2 | Schema, index, and loading products | 15 min |
| 3 | Vector search | 10 min |
| 4 | Filtered vector search | 10 min |
| 5 | Hybrid search with `FT.HYBRID` | 15 min |
| 6 | Faceting with aggregations | 10 min |
| 7 | Tuning knobs: experiment, observe, measure | 15 min |
| 8 | The optimizer study: pick a winner | 15 min |
| 9 | Wrap-up and next experiments | 5 min |

Lab instructions live in [`labs/`](labs/) (present on every branch).

### One branch to work on, the rest are safety nets

Check out **`lab-1-starter`** once and stay on it: it contains the guided
gaps for *all* of Labs 1–6, and you fill them in one lab at a time —
no branch switching during the workshop. The other branches exist for
recovery and comparison:

- **`lab-N-solution`** — the reference implementation through Lab N.
  Ask "what am I missing?" with `git diff lab-3-solution -- internal/`,
  or check it out if you're stuck.
- **`lab-N-starter`** (N ≥ 2) — clean re-entry points: joining late or
  starting over? `git checkout lab-4-starter` gives you Labs 1–3 already
  solved.
- **`workshop-complete`** (also `lab-7`, `lab-8`) — the finished service,
  used for Labs 7–9, which add configuration and measurement rather than
  code.

## 🚀 Getting Started

1. Pick a setup option from the prerequisites and open the project
   (Codespace, Dev Container, or local clone).

2. Verify the tools and start Redis:

   ```bash
   make doctor
   make redis-start
   ```

3. Start with [Lab 0](labs/lab-0.md):

   ```bash
   git checkout lab-1-starter
   ```

## 🧭 How the Repository Is Laid Out

```
cmd/prep         WANDS download + deterministic workshop sample (provided)
cmd/searchd      the service: JSON API + web UI on one port
cmd/eval         nDCG@10 / Recall@25 evaluator (Lab 7's instrument)
internal/embed   ← Lab 1 lives here
internal/search  ← Labs 2–6 live here
internal/httpapi HTTP plumbing (provided — you never edit this)
web/             the search UI (provided — served via go:embed)
optimizer/       Lab 8's Retrieval Optimizer study (Python, uv)
config.yaml      every tuning knob in the workshop
labs/            lab instructions
```

The design rule: **every line you write is RedisVL code.** HTTP handlers,
JSON, config, and frontend are all provided.

## 📚 Resources

- [RedisVL for Golang](https://github.com/redis-developer/redis-vl-golang) ·
  [documentation](https://redis-developer.github.io/redis-vl-golang/redisvl/current/)
- [Redis Search](https://redis.io/docs/latest/develop/interact/search-and-query/)
- [WANDS dataset](https://github.com/wayfair/WANDS)
- [Redis Retrieval Optimizer](https://github.com/redis-applied-ai/redis-retrieval-optimizer)

## 🤝 Contributing

Contributions are welcome! Please feel free to submit a Pull Request. For major
changes, please open an issue first to discuss what you would like to change.

## 👥 Maintainers

- Ricardo Ferreira — [@riferrei](https://github.com/riferrei)

## 📄 License

This project is licensed under the [MIT License](LICENSE).

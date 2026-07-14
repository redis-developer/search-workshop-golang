# Lab 1 — Local Embeddings with a Redis Cache

**Duration:** ~15 minutes · **Branch:** `lab-1-starter` · **Solution:** `lab-1-solution`

## Goal

Give the service the ability to turn text into vectors — locally, in-process,
with no API keys — and make repeated embeddings free with a Redis-backed
cache.

## Concepts

- **Embeddings** map text to points in vector space where semantic similarity
  becomes geometric proximity. Products and queries embedded with the *same
  model* become comparable.
- **Local inference:** RedisVL for Golang's `hf` module downloads a
  sentence-transformer from the Hugging Face Hub once, then runs it through
  ONNX Runtime. Offline-capable, key-free, and its output is verified
  byte-for-byte against Python sentence-transformers.
- **Cache-aside:** embedding the same text twice is wasted compute. The
  `EmbeddingsCache` stores vectors in Redis keyed by `(content, model)` —
  so switching models never serves stale vectors.

## Your task

All in `internal/embed/embed.go`:

1. **`newProvider`** — construct the configured vectorizer:
   - `config.ProviderHF` → `hf.New(ctx, hf.Config{...})` with the model,
     batch size, and `onnxRuntimePath()` (provided) from the config.
   - `config.ProviderOpenAI` → `vectorize.NewOpenAIVectorizer(ctx, ...)`.
2. **`New`** — wrap the provider with cache-aside caching:
   `cache.NewEmbeddingsCache(client, ...)` named `cfg.CacheName()`, then
   `cache.NewCachedVectorizer(inner, embCache)`.

Both constructors probe the model's dimensionality — you'll see `Dims()`
drive the index schema in Lab 2.

## Checkpoint

```bash
make run
```

Startup now reaches the *next* missing piece (Lab 2's index), but first it
prints:

```
embedding 600 products with sentence-transformers/all-MiniLM-L6-v2 (cached after the first run)...
embedded in 40.0s
```

Stop and rerun it: `embedded in 0.1s`. That difference is your cache working —
check it in Redis: `docker exec -it workshop-redis redis-cli KEYS 'embedcache-*'`
(then `HGETALL` one of them).

```bash
make verify LAB=1
```

## Troubleshooting

- `Error loading ONNX shared library` — install ONNX Runtime
  (`brew install onnxruntime` / see `make doctor`), or set
  `ONNXRUNTIME_LIB_PATH` to the library file.
- Model download slow? It happens once; the Hub cache lives under your user
  cache directory (`redisvl-go/models`).

Next: [Lab 2 — Schema, index, and loading products](lab-2.md)

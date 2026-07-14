# Lab 1 — Local Embeddings with a Redis Cache

**Duration:** ~15 minutes

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

Everything happens in **`internal/embed/embed.go`**, in three copy-paste
steps. (Read the code as you paste it — the point is understanding what it
does, not typing it.)

**Step 1** — replace the `import` block at the top of the file with:

```go
import (
	"context"
	"fmt"
	"os"

	"github.com/redis/go-redis/v9"

	"github.com/redis-developer/redis-vl-golang/extensions/cache"
	"github.com/redis-developer/redis-vl-golang/extensions/vectorize"
	hf "github.com/redis-developer/redis-vl-golang/extensions/vectorize/hf"

	"github.com/redis-developer/search-workshop-golang/internal/config"
)
```

**Step 2** — replace the entire `New` function with:

```go
// New builds the configured vectorizer and wraps it in an EmbeddingsCache
// stored on client. The cache is keyed by (content, model), so switching
// models in config.yaml never serves stale vectors.
func New(ctx context.Context, cfg *config.Config, client redis.UniversalClient) (*Vectorizer, error) {
	// LAB 1 (reference solution): construct the configured embedding
	// provider, then wrap it with cache-aside caching.
	inner, err := newProvider(ctx, cfg)
	if err != nil {
		return nil, err
	}

	embCache := cache.NewEmbeddingsCache(client, cache.EmbeddingsCacheOptions{
		Name: cfg.CacheName(),
	})
	cached := cache.NewCachedVectorizer(inner, embCache)

	v := &Vectorizer{Vectorizer: cached, inner: inner}
	if closer, ok := inner.(interface{ Close() error }); ok {
		v.closer = closer.Close
	}
	return v, nil
}
```

**Step 3** — replace the entire `newProvider` function with:

```go
// newProvider constructs the raw (uncached) vectorizer for the configured
// provider.
func newProvider(ctx context.Context, cfg *config.Config) (vectorize.Vectorizer, error) {
	switch cfg.Embedding.Provider {
	case config.ProviderHF:
		// Local in-process embeddings: the model is downloaded from the
		// Hugging Face Hub once, cached on disk, and executed through
		// ONNX Runtime. No API key, no per-call network access.
		return hf.New(ctx, hf.Config{
			Model:           cfg.Embedding.Model,
			BatchSize:       cfg.Embedding.BatchSize,
			ONNXRuntimePath: onnxRuntimePath(),
		})
	case config.ProviderOpenAI:
		// Hosted embeddings: requires OPENAI_API_KEY in the environment.
		return vectorize.NewOpenAIVectorizer(ctx, vectorize.OpenAIConfig{
			Model:     cfg.Embedding.Model,
			BatchSize: cfg.Embedding.BatchSize,
		})
	default:
		return nil, fmt.Errorf("unknown embedding provider %q", cfg.Embedding.Provider)
	}
}
```

Leave `onnxRuntimePath` and everything else untouched. Both constructors
probe the model's dimensionality — you'll see `Dims()` drive the index
schema in Lab 2.

## Checkpoint

```bash
make run
```

The first run downloads the model, then prints:

```
embedding 600 products with sentence-transformers/all-MiniLM-L6-v2 (cached after the first run)...
embedded in 40.0s
```

The service then keeps serving, and searches now fail with the *next*
missing piece: `LAB 2: index creation and loading not implemented`.
Progress!

Stop it (Ctrl-C) and rerun: `embedded in 0.1s`. That difference is your
cache working — see it in Redis:
`docker exec -it workshop-redis redis-cli KEYS 'embedcache-*'`
(then `HGETALL` one of them).

With `make run` still running:

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

// Package embed builds the workshop's text vectorizer: a local ONNX
// sentence-transformer (hf) or the OpenAI embeddings API, wrapped in a
// Redis-backed embeddings cache so repeated texts are embedded only once.
//
// This is the code Lab 1 implements.
package embed

import (
	"context"
	"fmt"
	"os"

	"github.com/redis/go-redis/v9"

	"github.com/redis/redis-vl-golang/extensions/cache"
	"github.com/redis/redis-vl-golang/extensions/vectorize"
	hf "github.com/redis/redis-vl-golang/extensions/vectorize/hf"

	"github.com/redis-developer/search-workshop-golang/internal/config"
)

// Vectorizer is the workshop vectorizer: the provider selected in
// config.yaml behind a cache-aside embeddings cache.
type Vectorizer struct {
	vectorize.Vectorizer
	inner  vectorize.Vectorizer
	closer func() error
}

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

// Close releases provider resources (the hf provider holds an ONNX
// Runtime session).
func (v *Vectorizer) Close() error {
	if v.closer != nil {
		return v.closer()
	}
	return nil
}

// onnxRuntimePath locates the ONNX Runtime shared library so workshop
// participants never have to set environment variables by hand. An
// explicit ONNXRUNTIME_LIB_PATH always wins; otherwise the standard
// Homebrew (macOS) and Linux install locations are probed.
func onnxRuntimePath() string {
	if v := os.Getenv("ONNXRUNTIME_LIB_PATH"); v != "" {
		return v
	}
	candidates := []string{
		// macOS: brew install onnxruntime
		"/opt/homebrew/lib/libonnxruntime.dylib", // Apple Silicon
		"/usr/local/lib/libonnxruntime.dylib",    // Intel
		// Linux (incl. the workshop devcontainer)
		"/usr/local/lib/libonnxruntime.so",
		"/usr/lib/libonnxruntime.so",
		"/usr/lib/x86_64-linux-gnu/libonnxruntime.so",
		"/usr/lib/aarch64-linux-gnu/libonnxruntime.so",
	}
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return "" // fall back to the hf module's default resolution
}

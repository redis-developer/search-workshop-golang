package search

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/redis/go-redis/v9"

	redisvl "github.com/redis-developer/redis-vl-golang"

	"github.com/redis-developer/search-workshop-golang/internal/catalog"
	"github.com/redis-developer/search-workshop-golang/internal/config"
	"github.com/redis-developer/search-workshop-golang/internal/embed"
)

// Service owns the vectorizer and the product index for the current
// configuration. It is shared by the HTTP handlers, cmd/eval, and the
// reindex path.
type Service struct {
	cfg   *config.Config
	vec   *embed.Vectorizer
	index *redisvl.SearchIndex
}

// New connects to Redis, builds the configured vectorizer, and prepares
// (but does not create) the configured index.
func New(ctx context.Context, cfg *config.Config) (*Service, error) {
	vec, err := embed.New(ctx, cfg, clientFromURL(cfg.Redis.URL))
	if err != nil {
		return nil, fmt.Errorf("building vectorizer: %w", err)
	}

	s, err := BuildSchema(cfg, vec.Dims())
	if err != nil {
		return nil, err
	}
	index, err := redisvl.NewSearchIndexFromURL(s, cfg.Redis.URL)
	if err != nil {
		return nil, fmt.Errorf("connecting index: %w", err)
	}
	return &Service{cfg: cfg, vec: vec, index: index}, nil
}

// clientFromURL builds the raw go-redis client used by the embeddings
// cache. Errors surface on first use; the URL is validated again by
// NewSearchIndexFromURL.
func clientFromURL(url string) redis.UniversalClient {
	opts, err := redis.ParseURL(url)
	if err != nil {
		opts = &redis.Options{Addr: "localhost:6379"}
	}
	return redis.NewClient(opts)
}

// Config returns the service configuration.
func (s *Service) Config() *config.Config { return s.cfg }

// Ready reports whether the configured index exists in Redis.
func (s *Service) Ready(ctx context.Context) bool {
	ok, err := s.index.Exists(ctx)
	return err == nil && ok
}

// Ping verifies Redis connectivity.
func (s *Service) Ping(ctx context.Context) error {
	return s.index.Client().Ping(ctx).Err()
}

// Close releases the vectorizer and the Redis connection.
func (s *Service) Close() error {
	err := s.vec.Close()
	if cerr := s.index.Close(); err == nil {
		err = cerr
	}
	return err
}

// EnsureIndex makes the configured index queryable (the code Labs 1-3
// build). When the index already exists and force is false it does
// nothing. Otherwise it:
//
//  1. reads the prepared corpus from data/,
//  2. embeds every product's search_text (Lab 1: the embeddings cache
//     makes repeat runs nearly free),
//  3. loads the products as Redis hashes under the config's key prefix
//     (Lab 2), and
//  4. creates the index for the configured vector algorithm (Lab 2).
//
// Indexes for different algorithms over the same embedding model share
// keys, so `make reindex-matrix` embeds each model once.
func (s *Service) EnsureIndex(ctx context.Context, force bool) error {
	if !force && s.Ready(ctx) {
		return nil
	}

	products, err := catalog.ReadCorpus("data/corpus.jsonl")
	if err != nil {
		return fmt.Errorf("reading corpus (run `make prep` first): %w", err)
	}

	// LAB 1 (reference solution): embed every product's search_text in
	// one cached batch call.
	texts := make([]string, len(products))
	for i, p := range products {
		texts[i] = p.SearchText
	}
	fmt.Fprintf(os.Stderr, "embedding %d products with %s (cached after the first run)...\n",
		len(texts), s.vec.ModelName())
	start := time.Now()
	embeddings, err := s.vec.EmbedMany(ctx, texts)
	if err != nil {
		return fmt.Errorf("embedding corpus: %w", err)
	}
	fmt.Fprintf(os.Stderr, "embedded in %.1fs\n", time.Since(start).Seconds())

	// LAB 2: convert products to hash records and index them:
	//   1. for each product, build a map[string]any with the Field*
	//      constants as keys, plus "product_id"; serialize the vector
	//      with vectors.ToBuffer(embeddings[i], vectors.Float32)
	//      (import "github.com/redis-developer/redis-vl-golang/vectors")
	//   2. s.index.Create(ctx, redisvl.CreateOptions{Overwrite: true})
	//   3. s.index.Load(ctx, records, redisvl.LoadOptions{
	//        IDField: "product_id", BatchSize: 200})
	// See labs/lab-2.md.
	_ = embeddings
	return fmt.Errorf("LAB 2: index creation and loading not implemented; see labs/lab-2.md")
}

// Product fetches one product record by ID.
func (s *Service) Product(ctx context.Context, id string) (map[string]any, error) {
	return s.index.Fetch(ctx, id)
}

// Stats summarizes the index for the /stats endpoint.
func (s *Service) Stats(ctx context.Context) (map[string]any, error) {
	info, err := s.index.Info(ctx)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"index_name":      s.index.Name(),
		"key_prefix":      s.index.Prefix(),
		"algorithm":       s.cfg.Index.Algorithm,
		"embedding_model": s.vec.ModelName(),
		"dims":            s.vec.Dims(),
		"num_docs":        info["num_docs"],
		"percent_indexed": info["percent_indexed"],
	}, nil
}

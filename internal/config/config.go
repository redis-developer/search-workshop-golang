// Package config loads the workshop's single tuning surface: config.yaml.
//
// Every knob the labs experiment with lives in that file. Environment
// variables override the file for deployment-specific values (REDIS_URL,
// WORKSHOP_RUN_ID), and cmd/searchd exposes -algorithm/-model flags so the
// Lab 8 index matrix can be scripted without editing the file.
package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Embedding providers.
const (
	ProviderHF     = "hf"
	ProviderOpenAI = "openai"
)

// Search strategies.
const (
	StrategyText   = "text"
	StrategyVector = "vector"
	StrategyHybrid = "hybrid"
)

// Fusion methods for hybrid search.
const (
	FusionRRF    = "rrf"
	FusionLinear = "linear"
)

// Config is the parsed config.yaml.
type Config struct {
	Redis struct {
		URL   string `yaml:"url"`
		RunID string `yaml:"run_id"`
	} `yaml:"redis"`

	Embedding struct {
		Provider  string `yaml:"provider"`
		Model     string `yaml:"model"`
		BatchSize int    `yaml:"batch_size"`
	} `yaml:"embedding"`

	Index struct {
		Algorithm string `yaml:"algorithm"`
		HNSW      struct {
			M              int `yaml:"m"`
			EfConstruction int `yaml:"ef_construction"`
			EfRuntime      int `yaml:"ef_runtime"`
		} `yaml:"hnsw"`
		SVS struct {
			Compression string `yaml:"compression"`
		} `yaml:"svs"`
	} `yaml:"index"`

	Search struct {
		DefaultStrategy string `yaml:"default_strategy"`
		K               int    `yaml:"k"`
		Hybrid          struct {
			Fusion      string  `yaml:"fusion"`
			Alpha       float64 `yaml:"alpha"`
			RRFWindow   int     `yaml:"rrf_window"`
			RRFConstant int     `yaml:"rrf_constant"`
		} `yaml:"hybrid"`
	} `yaml:"search"`

	Server struct {
		Addr string `yaml:"addr"`
	} `yaml:"server"`
}

// Load reads and validates path, applying environment overrides.
func Load(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	// Environment overrides for deployment-specific values.
	if v := os.Getenv("REDIS_URL"); v != "" {
		cfg.Redis.URL = v
	}
	if v := os.Getenv("WORKSHOP_RUN_ID"); v != "" {
		cfg.Redis.RunID = v
	}

	cfg.applyDefaults()
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &cfg, nil
}

func (c *Config) applyDefaults() {
	if c.Redis.URL == "" {
		c.Redis.URL = "redis://localhost:6379"
	}
	if c.Redis.RunID == "" {
		c.Redis.RunID = "local"
	}
	if c.Embedding.Provider == "" {
		c.Embedding.Provider = ProviderHF
	}
	if c.Embedding.Model == "" {
		switch c.Embedding.Provider {
		case ProviderOpenAI:
			c.Embedding.Model = "text-embedding-3-small"
		default:
			c.Embedding.Model = "sentence-transformers/all-MiniLM-L6-v2"
		}
	}
	if c.Embedding.BatchSize <= 0 {
		c.Embedding.BatchSize = 32
	}
	if c.Index.Algorithm == "" {
		c.Index.Algorithm = "flat"
	}
	c.Index.Algorithm = strings.ToLower(c.Index.Algorithm)
	if c.Index.HNSW.M <= 0 {
		c.Index.HNSW.M = 16
	}
	if c.Index.HNSW.EfConstruction <= 0 {
		c.Index.HNSW.EfConstruction = 200
	}
	if c.Index.HNSW.EfRuntime <= 0 {
		c.Index.HNSW.EfRuntime = 10
	}
	if c.Index.SVS.Compression == "" {
		c.Index.SVS.Compression = "LVQ8"
	}
	if c.Search.DefaultStrategy == "" {
		c.Search.DefaultStrategy = StrategyVector
	}
	if c.Search.K <= 0 {
		c.Search.K = 10
	}
	if c.Search.Hybrid.Fusion == "" {
		c.Search.Hybrid.Fusion = FusionRRF
	}
	if c.Search.Hybrid.Alpha <= 0 || c.Search.Hybrid.Alpha >= 1 {
		c.Search.Hybrid.Alpha = 0.65
	}
	if c.Search.Hybrid.RRFWindow <= 0 {
		c.Search.Hybrid.RRFWindow = 20
	}
	if c.Search.Hybrid.RRFConstant <= 0 {
		c.Search.Hybrid.RRFConstant = 60
	}
	if c.Server.Addr == "" {
		c.Server.Addr = ":8081"
	}
}

func (c *Config) validate() error {
	switch c.Embedding.Provider {
	case ProviderHF, ProviderOpenAI:
	default:
		return fmt.Errorf("embedding.provider must be %q or %q, got %q",
			ProviderHF, ProviderOpenAI, c.Embedding.Provider)
	}
	switch c.Index.Algorithm {
	case "flat", "hnsw", "svs-vamana":
	default:
		return fmt.Errorf(`index.algorithm must be "flat", "hnsw", or "svs-vamana", got %q`, c.Index.Algorithm)
	}
	switch c.Search.DefaultStrategy {
	case StrategyText, StrategyVector, StrategyHybrid:
	default:
		return fmt.Errorf(`search.default_strategy must be "text", "vector", or "hybrid", got %q`, c.Search.DefaultStrategy)
	}
	switch c.Search.Hybrid.Fusion {
	case FusionRRF, FusionLinear:
	default:
		return fmt.Errorf(`search.hybrid.fusion must be "rrf" or "linear", got %q`, c.Search.Hybrid.Fusion)
	}
	return nil
}

// ModelSlug is a short, index-name-safe identifier for the embedding model
// (e.g. "sentence-transformers/all-MiniLM-L6-v2" -> "all-minilm-l6-v2").
func (c *Config) ModelSlug() string {
	slug := c.Embedding.Model
	if i := strings.LastIndex(slug, "/"); i >= 0 {
		slug = slug[i+1:]
	}
	return strings.ToLower(slug)
}

// KeyPrefix namespaces product keys by run and embedding model. Indexes
// with different vector algorithms but the same model share these keys, so
// the Lab 8 matrix loads each model's embeddings only once.
func (c *Config) KeyPrefix() string {
	return fmt.Sprintf("wands-%s-%s", c.Redis.RunID, c.ModelSlug())
}

// IndexName identifies one (model, algorithm) combination.
func (c *Config) IndexName() string {
	return fmt.Sprintf("%s-%s", c.KeyPrefix(), c.Index.Algorithm)
}

// CacheName namespaces the embeddings cache by run, so classroom
// participants sharing one Redis never collide.
func (c *Config) CacheName() string {
	return fmt.Sprintf("embedcache-%s", c.Redis.RunID)
}

// Describe is the compact configuration string surfaced in the UI's
// resultType line and the JSON meta block.
func (c *Config) Describe(strategy string) string {
	s := fmt.Sprintf("%s · %s · %s", strategy, c.Index.Algorithm, c.ModelSlug())
	if strategy == StrategyHybrid {
		if c.Search.Hybrid.Fusion == FusionLinear {
			s = fmt.Sprintf("%s/linear α=%.2f · %s · %s",
				strategy, c.Search.Hybrid.Alpha, c.Index.Algorithm, c.ModelSlug())
		} else {
			s = fmt.Sprintf("%s/rrf · %s · %s", strategy, c.Index.Algorithm, c.ModelSlug())
		}
	}
	return s
}

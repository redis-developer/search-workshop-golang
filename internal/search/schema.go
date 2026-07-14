// Package search is the RedisVL heart of the workshop service: the index
// schema, the index lifecycle (embed, load, create), and the query
// strategies the labs implement one by one.
package search

import (
	"fmt"

	"github.com/redis-developer/redis-vl-golang/schema"

	"github.com/redis-developer/search-workshop-golang/internal/config"
)

// Field names in the product index. The searchable text and the vector are
// both derived from search_text; the rest are filterable/facetable
// attributes.
const (
	FieldSearchText    = "search_text"
	FieldName          = "product_name"
	FieldClass         = "product_class"
	FieldHierarchy     = "category_hierarchy"
	FieldDescription   = "product_description"
	FieldFeatures      = "product_features"
	FieldAverageRating = "average_rating"
	FieldRatingCount   = "rating_count"
	FieldReviewCount   = "review_count"
	FieldEmbedding     = "embedding"
)

// BuildSchema defines the product index for the configured vector
// algorithm (this is the code Lab 2 implements).
//
// Text fields carry lexical search, the tag field carries exact
// category filtering and faceting, numeric fields carry range filters,
// and the vector field carries semantic search. dims comes from the
// configured embedding model, so switching models in config.yaml
// automatically re-dimensions the index.
func BuildSchema(cfg *config.Config, dims int) (*schema.IndexSchema, error) {
	attrs := schema.VectorAttrs{
		Dims:           dims,
		Datatype:       "float32",
		DistanceMetric: schema.Cosine,
	}
	switch cfg.Index.Algorithm {
	case "flat":
		attrs.Algorithm = schema.Flat
	case "hnsw":
		attrs.Algorithm = schema.HNSW
		attrs.M = intPtr(cfg.Index.HNSW.M)
		attrs.EfConstruction = intPtr(cfg.Index.HNSW.EfConstruction)
		attrs.EfRuntime = intPtr(cfg.Index.HNSW.EfRuntime)
	case "svs-vamana":
		attrs.Algorithm = schema.SVSVamana
		attrs.Compression = schema.Compression(cfg.Index.SVS.Compression)
	default:
		return nil, fmt.Errorf("unknown index algorithm %q", cfg.Index.Algorithm)
	}

	vectorField, err := schema.NewVectorField(FieldEmbedding, attrs)
	if err != nil {
		return nil, fmt.Errorf("defining vector field: %w", err)
	}

	return schema.NewIndexSchema(
		schema.IndexInfo{
			Name:     cfg.IndexName(),
			Prefixes: []string{cfg.KeyPrefix()},
		},
		schema.NewTextField(FieldSearchText),
		schema.NewTextField(FieldName),
		schema.NewTagField(FieldClass),
		schema.NewNumericField(FieldAverageRating),
		schema.NewNumericField(FieldRatingCount),
		schema.NewNumericField(FieldReviewCount),
		vectorField,
	)
}

func intPtr(n int) *int { return &n }

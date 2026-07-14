// Package search is the RedisVL heart of the workshop service: the index
// schema, the index lifecycle (embed, load, create), and the query
// strategies the labs implement one by one.
package search

import (
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
	// LAB 2: build the full product schema. Field types:
	//
	//   search_text, product_name                          -> schema.NewTextField
	//   product_class                                      -> schema.NewTagField
	//   average_rating, rating_count, review_count         -> schema.NewNumericField
	//   embedding                                          -> schema.NewVectorField with
	//     schema.VectorAttrs{Dims: dims, Datatype: "float32",
	//                        DistanceMetric: schema.Cosine, Algorithm: ...}
	//
	// The algorithm comes from cfg.Index.Algorithm:
	//   "flat"       -> schema.Flat
	//   "hnsw"       -> schema.HNSW, plus M/EfConstruction/EfRuntime from
	//                   cfg.Index.HNSW (use intPtr(...) for the *int attrs)
	//   "svs-vamana" -> schema.SVSVamana, plus
	//                   Compression: schema.Compression(cfg.Index.SVS.Compression)
	//
	// See labs/lab-2.md. The minimal schema below only exists so the
	// service starts during Lab 1 — replace it entirely.
	return schema.NewIndexSchema(
		schema.IndexInfo{
			Name:     cfg.IndexName(),
			Prefixes: []string{cfg.KeyPrefix()},
		},
		schema.NewTextField(FieldSearchText),
	)
}

func intPtr(n int) *int { return &n }

# Lab 2 — Schema, Index, and Loading Products

**Duration:** ~15 minutes

## Goal

Define the product index schema, create the index, and load the 600 embedded
products into Redis as hashes. After this lab the service starts clean and
`/stats` reports a queryable index.

## Concepts

- **Schema-driven indexing:** RedisVL indexes are declared as typed fields.
  Each type answers a different kind of question:

  | Field | Type | Question it answers |
  | --- | --- | --- |
  | `search_text` | text | lexical relevance (BM25) |
  | `product_name` | text | name-targeted matches |
  | `product_class` | tag | exact category filtering + facets |
  | `average_rating`, `rating_count`, `review_count` | numeric | range filters |
  | `embedding` | vector | semantic similarity |

- **`search_text`** was assembled by `cmd/prep` from name + class + category
  hierarchy + description + features — one field that both BM25 and the
  embedding model consume.
- **Vector algorithms** are declared in the schema (`FLAT` today; `HNSW` and
  `SVS-VAMANA` are one config edit away — that's Lab 7).
- **Index-first:** we create the index before loading, so Redis indexes
  documents as they arrive.

## Your task

Two files, four copy-paste steps.

**Step 1** — in **`internal/search/schema.go`**, replace the `import`
block with:

```go
import (
	"fmt"

	"github.com/redis-developer/redis-vl-golang/schema"

	"github.com/redis-developer/search-workshop-golang/internal/config"
)
```

**Step 2** — in the same file, replace the entire `BuildSchema` function
with:

```go
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
```

**Step 3** — in **`internal/search/service.go`**, replace the `import`
block with:

```go
import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/redis/go-redis/v9"

	redisvl "github.com/redis-developer/redis-vl-golang"
	"github.com/redis-developer/redis-vl-golang/vectors"

	"github.com/redis-developer/search-workshop-golang/internal/catalog"
	"github.com/redis-developer/search-workshop-golang/internal/config"
	"github.com/redis-developer/search-workshop-golang/internal/embed"
)
```

**Step 4** — in the same file, replace everything in `EnsureIndex` **from
the `// LAB 2:` comment to the end of the function** with:

```go
	// LAB 2 (reference solution): convert products to hash records with
	// the vector as a float32 buffer, load them, and create the index.
	records := make([]map[string]any, len(products))
	for i, p := range products {
		blob, err := vectors.ToBuffer(embeddings[i], vectors.Float32)
		if err != nil {
			return fmt.Errorf("serializing embedding for product %s: %w", p.ID, err)
		}
		records[i] = map[string]any{
			"product_id":         p.ID,
			FieldSearchText:      p.SearchText,
			FieldName:            p.Name,
			FieldClass:           p.Class,
			FieldHierarchy:       p.Hierarchy,
			FieldDescription:     p.Description,
			FieldFeatures:        p.Features,
			FieldAverageRating:   p.AverageRating,
			FieldRatingCount:     p.RatingCount,
			FieldReviewCount:     p.ReviewCount,
			FieldEmbedding:       blob,
		}
	}

	if err := s.index.Create(ctx, redisvl.CreateOptions{Overwrite: true}); err != nil {
		return fmt.Errorf("creating index %s: %w", s.index.Name(), err)
	}
	if _, err := s.index.Load(ctx, records, redisvl.LoadOptions{IDField: "product_id", BatchSize: 200}); err != nil {
		return fmt.Errorf("loading products: %w", err)
	}
	fmt.Fprintf(os.Stderr, "index %s ready: %d products\n", s.index.Name(), len(records))
	return nil
}
```

Note the naming scheme: keys are `wands-{run}-{model}:{product_id}` and the
index is `wands-{run}-{model}-{algorithm}` — so indexes with different
algorithms *share* keys for the same model. Lab 8 exploits this.

## Checkpoint

```bash
make run          # starts clean now
curl -s localhost:8081/stats
```

`/stats` should report `num_docs: 600` for `wands-local-all-minilm-l6-v2-flat`.
Fetch a single product — grab any `product_id` from the first line of
`data/corpus.jsonl`, then:

```bash
curl -s localhost:8081/products/<that id>
```

Inspect the raw hash in Redis too: `docker exec -it workshop-redis
redis-cli HGETALL wands-local-all-minilm-l6-v2:<that id>`.

```bash
make verify LAB=2
```

Searching in the UI still fails — the strategies are Labs 3–5. But the shelf
is stocked.

Next: [Lab 3 — Vector search](lab-3.md)

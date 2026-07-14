# Lab 2 — Schema, Index, and Loading Products

**Duration:** ~15 minutes · **Branch:** `lab-2-starter` · **Solution:** `lab-2-solution`

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

1. **`BuildSchema`** in `internal/search/schema.go` — build the
   `schema.IndexSchema` from the table above with
   `schema.NewIndexSchema(schema.IndexInfo{Name: cfg.IndexName(), Prefixes:
   []string{cfg.KeyPrefix()}}, ...fields)`. The vector field takes
   `schema.VectorAttrs{Dims: dims, Datatype: "float32", DistanceMetric:
   schema.Cosine, Algorithm: ...}` — switch on `cfg.Index.Algorithm` (the
   HNSW/SVS branches are spelled out in the starter comments).
2. **`EnsureIndex`** in `internal/search/service.go` — the loading half:
   convert each product to a `map[string]any` (the vector serialized with
   `vectors.ToBuffer(embeddings[i], vectors.Float32)`), then
   `index.Create(ctx, redisvl.CreateOptions{Overwrite: true})` and
   `index.Load(ctx, records, redisvl.LoadOptions{IDField: "product_id",
   BatchSize: 200})`.

Note the naming scheme: keys are `wands-{run}-{model}:{product_id}` and the
index is `wands-{run}-{model}-{algorithm}` — so indexes with different
algorithms *share* keys for the same model. Lab 8 exploits this.

## Checkpoint

```bash
make run          # starts clean now
curl -s localhost:8081/stats
curl -s localhost:8081/products/42
```

`/stats` should report `num_docs: 600` for `wands-local-all-minilm-l6-v2-flat`.
Inspect the raw data too: `docker exec -it workshop-redis redis-cli HGETALL
wands-local-all-minilm-l6-v2:42`.

```bash
make verify LAB=2
```

Searching in the UI still fails — the strategies are Labs 3–5. But the shelf
is stocked.

Next: [Lab 3 — Vector search](lab-3.md)

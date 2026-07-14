# Lab 6 — Faceting with Aggregations

**Duration:** ~10 minutes · **Catch-up branch:** `lab-6-starter` · **Solution:** `lab-6-solution`

## Goal

Answer catalog-level questions with `FT.AGGREGATE`: how many products per
category, and how well is each rated? This is the machinery behind every
ecommerce sidebar (“Sofas (127) · Beds (89) · …”).

## Concepts

- **Search vs. aggregation:** `FT.SEARCH` returns documents; `FT.AGGREGATE`
  returns *computed rows* — group, count, average, sort, all inside Redis.
- **The pipeline** you'll build:

  ```
  FT.AGGREGATE <index> *
    GROUPBY 1 @product_class
    REDUCE COUNT 0 AS count
    REDUCE AVG 1 @average_rating AS avg_rating
    SORTBY 2 @count DESC
    LIMIT 0 <limit>
  ```

- **The extension point:** RedisVL for Golang executes any type implementing
  `query.AggregationQuery` — one method, `AggregateArgs(indexName) []any`.
  You are about to write your first custom aggregation.

## Your task

Two copy-paste steps, both in **`internal/search/queries.go`**.

**Step 1** — replace the entire `AggregateArgs` method with:

```go
// AggregateArgs implements query.AggregationQuery.
func (q facetQuery) AggregateArgs(indexName string) []any {
	return []any{
		"FT.AGGREGATE", indexName, "*",
		"GROUPBY", 1, "@" + FieldClass,
		"REDUCE", "COUNT", 0, "AS", "count",
		"REDUCE", "AVG", 1, "@" + FieldAverageRating, "AS", "avg_rating",
		"SORTBY", 2, "@count", "DESC",
		"LIMIT", 0, q.limit,
		"DIALECT", 2,
	}
}
```

Note the arity numbers after `GROUPBY`, `REDUCE`, and `SORTBY` — they count
the arguments that follow, and getting them wrong is the classic
`FT.AGGREGATE` mistake.

**Step 2** — replace the entire `Facets` function with:

```go
// Facets aggregates the catalog by product class.
func (s *Service) Facets(ctx context.Context, limit int) ([]Facet, error) {
	if limit <= 0 {
		limit = 25
	}
	rows, err := s.index.Aggregate(ctx, facetQuery{limit: limit})
	if err != nil {
		return nil, fmt.Errorf("aggregating facets: %w", err)
	}
	facets := make([]Facet, 0, len(rows))
	for _, row := range rows {
		facets = append(facets, Facet{
			Class:     asString(row[FieldClass]),
			Count:     int64(asFloat(row["count"])),
			AvgRating: asFloat(row["avg_rating"]),
		})
	}
	return facets, nil
}
```

## Checkpoint

```bash
curl -s 'localhost:8081/facets' | jq '.facets[:5]'
```

Expect the sample's biggest classes — Beds, Kitchen Mats, Bar Stools,
Office Chairs — with counts and average ratings, largest first. Cross-check
one against a filtered search:

```bash
curl -s 'localhost:8081/facets' | jq '.facets[0]'
curl -s 'localhost:8081/search?query=bed&class=Beds&k=25' | jq '.matchedProducts | length'
```

```bash
make verify LAB=6
```

**The build is complete.** Every query pattern from the scope is live: vector,
filtered vector, text, hybrid, hybrid+filtered, facets. From here on you
change *configuration*, not code.

Next: [Lab 7 — Tuning knobs](lab-7.md)

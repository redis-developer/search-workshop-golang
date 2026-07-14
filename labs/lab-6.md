# Lab 6 — Faceting with Aggregations

**Duration:** ~10 minutes · **Branch:** `lab-6-starter` · **Solution:** `lab-6-solution`

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

In `internal/search/queries.go`:

1. **`facetQuery.AggregateArgs`** — return the argument slice for the
   pipeline above (mind the arity numbers after GROUPBY/REDUCE/SORTBY).
2. **`Facets`** — execute via `s.index.Aggregate(ctx, facetQuery{limit})`
   and map each row to a `Facet{Class, Count, AvgRating}` (rows are keyed
   `product_class`, `count`, `avg_rating`).

## Checkpoint

```bash
curl -s 'localhost:8081/facets' | jq '.facets[:5]'
```

Expect the sample's biggest classes with counts and average ratings, largest
first. Cross-check one against a filter search:

```bash
curl -s 'localhost:8081/facets' | jq '.facets[0]'
curl -s 'localhost:8081/search?query=*anything*&class=<that class>&k=25' | jq '.matchedProducts | length'
```

```bash
make verify LAB=6
```

**The build is complete.** Every query pattern from the scope is live: vector,
filtered vector, text, hybrid, hybrid+filtered, facets. From here on you
change *configuration*, not code.

Next: [Lab 7 — Tuning knobs](lab-7.md)

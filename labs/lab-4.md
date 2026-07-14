# Lab 4 — Filtered Vector Search

**Duration:** ~10 minutes · **Branch:** `lab-4-starter` · **Solution:** `lab-4-solution`

## Goal

Combine semantic similarity with hard catalog constraints: category, minimum
rating, minimum review count. Similarity finds *what you mean*; filters
enforce *what you require*.

## Concepts

- **Pre-filtering:** Redis applies the filter *before* KNN, so you always get
  (up to) `k` results that satisfy the constraints — not `k` results
  awkwardly post-filtered down to two.
- **The filter DSL:** RedisVL composes typed filters into one expression:

  ```go
  filter.Tag(FieldClass).Eq("Sofas")               // @product_class:{Sofas}
  filter.Num(FieldAverageRating).Ge(4)             // @average_rating:[4 +inf]
  expr1.And(expr2, expr3)                          // intersection
  ```

  The rendered query strings are identical to what RedisVL for Python
  produces — this DSL is a port, not a reinvention.

## Your task

Two copy-paste steps, both in **`internal/search/queries.go`**.

**Step 1** — replace the entire `buildFilter` function with:

```go
// buildFilter combines the request's catalog constraints into one filter
// expression (Lab 4). Returns nil when unfiltered.
func buildFilter(req Request) *filter.Expression {
	var exprs []*filter.Expression
	if req.Class != "" {
		exprs = append(exprs, filter.Tag(FieldClass).Eq(req.Class))
	}
	if req.MinRating > 0 {
		exprs = append(exprs, filter.Num(FieldAverageRating).Ge(req.MinRating))
	}
	if req.MinReviews > 0 {
		exprs = append(exprs, filter.Num(FieldReviewCount).Ge(float64(req.MinReviews)))
	}
	if len(exprs) == 0 {
		return nil
	}
	out := exprs[0]
	if len(exprs) > 1 {
		out = out.And(exprs[1:]...)
	}
	return out
}
```

**Step 2** — in `searchVector`, replace the two lines

```go
	// LAB 4: when f is non-nil, pre-filter the KNN candidates with
	// q.Filter(f). See labs/lab-4.md.
	_ = f
```

with:

```go
	if f != nil {
		// LAB 4 (reference solution): pre-filter the KNN candidates.
		q.Filter(f)
	}
```

## Checkpoint

```bash
# similar sofas, but only well-reviewed ones
curl -s 'localhost:8081/search?query=outdoor+sofa&min_rating=4&min_reviews=5' | jq '.meta.filtered, [.matchedProducts[].rating]'

# constrain to one category
curl -s 'localhost:8081/search?query=comfortable+seat&class=Office+Chairs' | jq '[.matchedProducts[].class]'
```

Every returned rating should be ≥ 4; every class should match. Try a filter
that's too tight (`min_reviews=10000`) — an empty result is correct behavior,
and exactly what a production API should return.

Now the interesting question: run `outdoor sofa` unfiltered and with
`min_rating=4`. The filtered results are more *trustworthy* — but did you
lose any highly relevant products that just lack reviews? Hold that thought:
Lab 8 measures this exact trade-off (`vector` vs `vector_filtered`).

```bash
make verify LAB=4
```

Next: [Lab 5 — Hybrid search](lab-5.md)

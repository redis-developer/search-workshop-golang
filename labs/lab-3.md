# Lab 3 — Vector Search

**Duration:** ~10 minutes · **Catch-up branch:** `lab-3-starter` · **Solution:** `lab-3-solution`

## Goal

The first results in the browser: semantic product search. Embed the user's
query with the *same model* that embedded the products, then ask Redis for the
nearest neighbors.

## Concepts

- **KNN vector search:** the query becomes a vector; Redis returns the `k`
  products whose embeddings are closest (cosine distance). No keyword needs
  to match — “couch” finds sofas.
- **Query-side caching:** the query is embedded through the same
  `CachedVectorizer` from Lab 1. The UI searches as you type (debounced), so
  repeated queries hit the cache — watch the timing drop in the footer.
- **`vector_distance`:** each result carries its distance; lower is closer.
  It surfaces as `score` in the JSON response.

## Your task

One copy-paste step: in **`internal/search/queries.go`**, replace the
entire `searchVector` function with:

```go
func (s *Service) searchVector(ctx context.Context, text string, f *filter.Expression, k int) ([]map[string]any, error) {
	// LAB 3 (reference solution): embed the query text, then run KNN.
	vec, err := s.vec.Embed(ctx, text)
	if err != nil {
		return nil, fmt.Errorf("embedding query: %w", err)
	}
	q := query.NewVectorQuery(FieldEmbedding, vec).
		NumResults(k).
		ReturnFields(returnFields...)
	// LAB 4: when f is non-nil, pre-filter the KNN candidates with
	// q.Filter(f). See labs/lab-4.md.
	_ = f
	return s.index.Query(ctx, q)
}
```

Read it top to bottom: embed the query with the *same cached vectorizer*
that embedded the products, build a KNN query on the embedding field,
execute. The filter argument is deliberately ignored — that's Lab 4.

## Checkpoint

Restart `make run`, open <http://localhost:8081>, and search
**`ergonomic chair`** — the query you're following.

Products appear — and look at *what* appears: office and task chairs,
including ones whose descriptions never contain the word “ergonomic”. The
footer reads something like:

```
vector · flat · all-minilm-l6-v2 → 14 ms
```

Try paraphrases that showcase semantics: `comfortable chair for long work
days`, `something soft to sit on`. Then try an exact-vocabulary query like
`anti fatigue mat` and compare it with a paraphrase (`standing desk floor
cushion`) — precise catalog terms are where pure vector search starts to
wobble. Remember that observation for Lab 5.

```bash
make verify LAB=3
curl -s 'localhost:8081/search?query=ergonomic+chair' | jq '.meta, .matchedProducts[0]'
```

(No `jq`? Drop the pipe — the raw JSON is small.)

Next: [Lab 4 — Filtered vector search](lab-4.md)

# Lab 5 — Hybrid Search with FT.HYBRID

**Duration:** ~15 minutes · **Branch:** `lab-5-starter` · **Solution:** `lab-5-solution`

## Goal

Fuse lexical and semantic retrieval in a single server-side command, and make
all three strategies switchable from the UI dropdown.

## Concepts

- **Why hybrid?** In Lab 3 you saw vector search shine on intent (`comfy
  couch`) and stumble on exactness (`all-clad 7 qt slow cooker`). BM25 has
  the opposite profile. Hybrid runs both legs and fuses the rankings.
- **`FT.HYBRID`** (Redis 8.4+) executes the text leg and the vector leg
  *inside Redis* and combines them server-side. No client-side merging, one
  round trip.
- **Fusion methods:**
  - **RRF** (reciprocal rank fusion): rank-based, ignores incomparable score
    scales — a robust default.
  - **Linear**: weighted score blend. `alpha` is the **text** weight —
    `alpha: 0.65` means 65% lexical, 35% semantic.
- **Text search** comes along for free: the dispatcher's `text` strategy is a
  plain `TextQuery` (BM25 over `search_text`) — hybrid's lexical leg on its
  own, and your baseline.

## Your task

**`searchHybrid`** in `internal/search/queries.go`:

1. Embed the query (same as Lab 3).
2. Build the query with both legs:

   ```go
   q := query.NewHybridQuery(text, FieldSearchText, vec, FieldEmbedding).
       NumResults(k).
       ReturnFields(returnFields...)
   ```

3. Attach the filter when non-nil (hybrid + filters compose!).
4. Apply the configured fusion: `q.CombineRRF(window, constant)` or
   `q.CombineLinear(alpha)` from `cfg.Search.Hybrid`.
5. Execute with `s.index.Hybrid(ctx, q)` — note: not `Query`; `FT.HYBRID`
   has its own execution path.

## Checkpoint

Restart, then work the dropdown on **one query at a time** and compare:

| Query | text | vector | hybrid |
| --- | --- | --- | --- |
| `all-clad 7 qt slow cooker` | nails it | drifts | nails it |
| `comfy couch for a small living room` | literal | gets it | gets it |
| `outdoor sofa` | decent | decent | best of both |

The footer shows the fusion in play: `hybrid/rrf · flat · all-minilm-l6-v2`.

```bash
curl -s 'localhost:8081/search?query=outdoor+sofa&strategy=hybrid' | jq .meta
make verify LAB=5
```

Which strategy is *actually* better? You've now formed an opinion from six
queries. Lab 7 replaces that opinion with numbers.

Next: [Lab 6 — Faceting](lab-6.md)

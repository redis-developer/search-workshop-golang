# Lab 7: Tuning Knobs

**Duration:** ~20 minutes · **No new code** (behind? `git checkout workshop-complete` gives you the finished app)

## Goal

Turn every knob the system has, and *observe* what each one changes. First
with your eyes (the UI), then with numbers (`make eval`).

## The knob surface

Everything lives in [`config.yaml`](../config.yaml). The loop is always:

```bash
vi config.yaml && make reindex && make run     # then re-run your query
```

…and the UI footer tells you which configuration answered
(`hybrid/rrf · hnsw · all-minilm-l6-v2 → 9 ms`).

### Knob 1: Index algorithm

| `index.algorithm` | What it is | When |
| --- | --- | --- |
| `flat` | exact, brute force | small corpora, baselines |
| `hnsw` | approximate graph | large catalogs, low latency |
| `svs-vamana` | graph + `LVQ8` **quantization/compression** | when vector memory is the constraint |

Try each. At 600 products you'll barely see latency move, and that's *itself* a
finding (approximation pays off at scale, not here). Compare memory instead:
`curl -s localhost:8081/stats`.

### Knob 2: Embedding model

```yaml
embedding:
  model: sentence-transformers/all-mpnet-base-v2   # 768 dims, ~4x slower
```

`make reindex` re-embeds (one-time, cached forever after). Are the results
*better*, or just different? Keep that question; it's exactly what Lab 8
answers. If you brought an `OPENAI_API_KEY`, try `provider: openai` with
`text-embedding-3-small` as a third contestant.

### Knob 3: Query strategy and fusion

```yaml
search:
  hybrid:
    fusion: linear     # then try alpha: 0.2 vs 0.8
```

Same query, `alpha: 0.2` vs `alpha: 0.8`: watch results reorder as you slide
between "mostly semantic" and "mostly lexical".

## From eyeballs to numbers

Your impressions so far are anecdotes from a handful of queries. The qrels
from Lab 0 let us do better:

```bash
make eval                          # scores the current config
go run ./cmd/eval -strategy text
go run ./cmd/eval -strategy vector
go run ./cmd/eval -strategy hybrid -v   # -v: per-query breakdown
```

Three numbers per configuration:

- **nDCG@10**: graded first-page quality (Exact=2 beats Partial=1, and rank
  position matters). The headline metric.
- **Recall@25**: of the known-relevant products, how many are in the top 25?
  WANDS queries often have *many* relevant products, so treat this as a
  coverage guardrail, not a grade.
- **avg query ms**: the tie-breaker.

Open `cmd/eval/main.go`: the metrics are ~60 readable lines. Nothing about
evaluation is magic.

## Watching the index: monitoring with Redis

Everything so far measured *your queries*. Production search also needs you
to watch *the index itself*, and Redis exposes all of it. The primary tool
is `FT.INFO`:

```bash
docker exec workshop-redis redis-cli FT.INFO wands-local-all-minilm-l6-v2-flat
```

The output is long; these are the fields worth knowing by name:

| Field | What it tells you | When to worry |
| --- | --- | --- |
| `num_docs` | documents in the index | drops or stalls during a load |
| `percent_indexed` | background indexing progress (1 = done) | stuck below 1 for long |
| `hash_indexing_failures` | documents Redis could not index | anything above 0 |
| `total_indexing_time` | cumulative indexing cost (ms) | trending up release over release |
| `vector_index_sz_mb` / memory fields | what your vectors actually cost | growth that outpaces the catalog |

Now watch a reindex happen live. In one terminal, start a rebuild with the
larger model (a heavier load, so there is something to watch):

```bash
go run ./cmd/searchd -reindex-only -model sentence-transformers/all-mpnet-base-v2
```

In another, poll the interesting fields while it runs:

```bash
for i in $(seq 1 15); do
  docker exec workshop-redis redis-cli FT.INFO wands-local-all-mpnet-base-v2-flat \
    | grep -A1 -E '^(num_docs|percent_indexed|hash_indexing_failures)$'
  echo ---; sleep 1
done
```

You will see `num_docs` climb and `percent_indexed` converge to 1. That
loop, pointed at production and feeding a dashboard, is the essence of
search monitoring. Two companions worth knowing:

- **Key-level cost:** `docker exec workshop-redis redis-cli MEMORY USAGE
  wands-local-all-minilm-l6-v2:<some product id>` shows what one embedded
  product costs; `INFO memory` shows the whole instance.
- **Redis Insight:** the GUI (installed as a VS Code extension in the
  devcontainer, or as a [desktop app](https://redis.io/insight/)) connects
  to `localhost:6379` and gives you the same index stats, key browsing,
  and a query profiler, no CLI required.

The app already exposes the service-side half of this story: `/healthz`
for liveness, `/stats` for index status, and `meta.query_ms` in every
search response as the latency signal you would export to your metrics
system.

## What to bring to Lab 8

You now have measured hunches: which strategy leads on nDCG, whether mpnet
earned its 4x cost, whether the index algorithm mattered at this scale. Lab 8
runs the *full matrix* so the hunches become a decision.

Next: [Lab 8: The optimizer study](lab-8.md)

# Lab 7 — Tuning Knobs: Experiment, Observe, Measure

**Duration:** ~15 minutes · **Branch:** `lab-7` (= `lab-6-solution`) · **No new code**

## Goal

Turn every knob the system has — and *observe* what each one changes. First
with your eyes (the UI), then with numbers (`make eval`).

## The knob surface

Everything lives in [`config.yaml`](../config.yaml). The loop is always:

```bash
vi config.yaml && make reindex && make run     # then re-run your query
```

…and the UI footer tells you which configuration answered
(`hybrid/rrf · hnsw · all-minilm-l6-v2 → 9 ms`).

### Knob 1 — Index algorithm

| `index.algorithm` | What it is | When |
| --- | --- | --- |
| `flat` | exact, brute force | small corpora, baselines |
| `hnsw` | approximate graph | large catalogs, low latency |
| `svs-vamana` | graph + `LVQ8` **quantization/compression** | when vector memory is the constraint |

Try each. At 600 products you'll barely see latency move — that's *itself* a
finding (approximation pays off at scale, not here). Compare memory instead:
`curl -s localhost:8081/stats`.

### Knob 2 — Embedding model

```yaml
embedding:
  model: sentence-transformers/all-mpnet-base-v2   # 768 dims, ~4x slower
```

`make reindex` re-embeds (one-time, cached forever after). Are the results
*better* — or just different? Keep that question; it's exactly what Lab 8
answers. If you brought an `OPENAI_API_KEY`, try `provider: openai` with
`text-embedding-3-small` as a third contestant.

### Knob 3 — Query strategy and fusion

```yaml
search:
  hybrid:
    fusion: linear     # then try alpha: 0.2 vs 0.8
```

Same query, `alpha: 0.2` vs `alpha: 0.8` — watch results reorder as you slide
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

- **nDCG@10** — graded first-page quality (Exact=2 beats Partial=1, and rank
  position matters). The headline metric.
- **Recall@25** — of the known-relevant products, how many are in the top 25?
  WANDS queries often have *many* relevant products, so treat this as a
  coverage guardrail, not a grade.
- **avg query ms** — the tie-breaker.

Open `cmd/eval/main.go` — the metrics are ~60 readable lines. Nothing about
evaluation is magic.

## What to bring to Lab 8

You now have measured hunches: which strategy leads on nDCG, whether mpnet
earned its 4x cost, whether the index algorithm mattered at this scale. Lab 8
runs the *full matrix* so the hunches become a decision.

Next: [Lab 8 — The optimizer study](lab-8.md)

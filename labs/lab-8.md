# Lab 8 — The Optimizer Study: Pick a Winner

**Duration:** ~15 minutes · **Branch:** `lab-8` (= `lab-7`) · **No new Go code**

## Goal

Run every meaningful configuration through one systematic study and read the
answer off a single ranked table: **which configuration goes to production?**

## Concepts

- **The index matrix.** Lab 2's naming scheme (`wands-{run}-{model}-{algo}`,
  keys shared per model) means one command builds every combination without
  re-embedding more than once per model:

  ```bash
  make reindex-matrix
  ```

  3 algorithms × MiniLM, plus 2 × mpnet = 5 indexes over 2 sets of keys.

- **The Redis Retrieval Optimizer** is a Python evaluation harness from the
  Redis ecosystem. Its built-in methods (bm25, vector, …) are fixed, but its
  `search_method_map` extension point lets us register *parameterized*
  methods — each name becomes one comparable row. We register ten: `bm25`,
  `vector`, `vector_filtered`, `hybrid_rrf`, `hybrid_rrf_filtered`, and a
  linear-weight sweep (`hybrid_linear_text_020` … `080`).

- **Polyglot on purpose.** The Go service *built* the indexes; the Python
  study *measures* them, reading the same qrels. Interoperability isn't a
  compromise — it's the demonstration that your Redis data model is the
  contract, not the client language.

## Steps

1. Build the matrix (mpnet's first embedding pass takes a minute or two):

   ```bash
   make reindex-matrix
   ```

2. One-time Python setup, then run the study:

   ```bash
   make study-deps
   make study
   ```

   ~50 rows later (5 indexes × 10 methods): one table, sorted by nDCG, then
   recall, then latency — first-page relevance first, coverage second, speed
   as tie-breaker. Also saved to `optimizer/.study/study-results.csv`.

## Reading the table

Work through it as a group. Questions the table answers with evidence:

- **Strategy:** does hybrid beat both of its legs? Which fusion — and does
  the linear alpha sweep actually matter, or is it flat within noise?
- **Model:** did mpnet (768d, ~4x embedding cost, ~2x memory) beat MiniLM?
  On this sample, bigger is *not* automatically better — check before paying.
- **Index type:** at 600 products, do FLAT / HNSW / SVS-VAMANA differ on
  quality? On memory (`total_memory_mb` — spot LVQ8's effect)? What would
  you expect at 43k products? (That's the full-WANDS take-home.)
- **Filters:** the `_filtered` rows trade nDCG for guaranteed review
  quality. Is that trade right for *your* product? (Trick question: it's a
  product decision — but now it's a *quantified* one.)

⚠️ 24 queries is a live-workshop sample. Differences beyond ~0.02 nDCG are
signal; smaller gaps may flip between runs. Full-WANDS mode
(`go run ./cmd/prep -refresh -full`) is the rigorous version.

Next: [Lab 9 — Wrap-up](lab-9.md)

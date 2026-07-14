# Lab 9: Wrap-Up and Next Experiments

**Duration:** ~5 minutes

## What you built

A production-shaped Go search service, not a notebook:

- Local, key-free embeddings (ONNX) behind a Redis embeddings cache
- A schema-driven index with text, tag, numeric, and vector fields
- Four live retrieval strategies (text / vector / filtered / hybrid) plus
  facets, switchable per request, observable per response
- An evaluation loop: graded human judgments → nDCG@10 / Recall@25 → one
  ranked table across strategies × models × index types

More important than the code: **the method.** Build → observe → measure →
decide. Every tuning debate in your team can now end with `make study`
instead of the loudest opinion.

## Stating the recommendation

A good wrap-up sentence has all four parts: configuration, evidence, cost,
and the next experiment. For example, from a typical run of this sample:

> “Ship **MiniLM + HNSW + hybrid/RRF** (nDCG 0.754, ~1.6 ms): hybrid beats
> both of its legs, mpnet didn't earn its 4x embedding cost, and HNSW ties
> FLAT here while being the only option that scales. Next experiment:
> validate on full WANDS (43k products), where the index-type differences
> should actually appear.”

Yours may differ, and that's the point. Read it off *your* table.

## Take-home experiments

1. **Full WANDS:** `go run ./cmd/prep -refresh -full`, then re-run the
   matrix and study against 42,994 products and every judged query. Watch
   the index-type rows separate.
2. **Cross-encoder reranking:** RedisVL for Golang ships a local
   `CrossEncoder` (same `hf` module you used in Lab 1). Retrieve 50 with
   hybrid, rerank to 10 with `ms-marco-MiniLM-L-6-v2`, and register it as an
   eleventh `search_method_map` entry. Does precision@10 justify the extra
   ~30 ms?
3. **Your own quality gates:** swap the workshop's `min_rating` filter for
   your real business constraints (in-stock, shippable, margin) and re-read
   the `_filtered` rows.
4. **A classroom of one Redis:** set `WORKSHOP_RUN_ID` per participant and
   point everyone at one Redis Cloud database; the namespacing you saw in
   `config.yaml` exists for exactly this.

## Where to go next

- [RedisVL for Golang documentation](https://redis-developer.github.io/redis-vl-golang/redisvl/current/)
- [Redis Retrieval Optimizer](https://github.com/redis-applied-ai/redis-retrieval-optimizer)
  for grid and Bayesian studies, beyond the search study used here
- [WANDS](https://github.com/wayfair/WANDS): the dataset's paper and full
  judgment methodology

Thank you for building, and measuring, with Redis. 🚀

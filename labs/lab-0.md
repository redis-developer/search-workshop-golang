# Lab 0 — Setup: Data, Redis, and the Empty Shelf

**Duration:** ~10 minutes · **Branch:** `lab-1-starter`

## Goal

Get the workshop environment running end to end: Redis up, the WANDS dataset
prepared, and the service serving its UI — with search still unimplemented.
Understanding what's *missing* is the map for the rest of the workshop.

## The scenario

You are improving product search for an ecommerce catalog. The dataset is
[WANDS](https://github.com/wayfair/WANDS), Wayfair's product-search relevance
dataset. It gives us three things, and keeping their roles separate matters
for everything that follows:

| Artifact | Contents | Role |
| --- | --- | --- |
| **corpus** | 600 product records: names, classes, descriptions, features, ratings | What we search |
| **queries** | 24 real user searches (“platform bed”, “outdoor couch”, …) | What users ask |
| **qrels** | Human judgments: is product P **Exact** (2), **Partial** (1), or **Irrelevant** (0) for query Q? | How we *measure* |

Most search demos stop at the corpus. The qrels are what turn this workshop
from “look, results!” into engineering.

## Steps

1. Verify your tools and start Redis:

   ```bash
   make doctor
   make redis-start
   ```

2. Download and prepare the workshop dataset (deterministic — everyone gets
   the same 600 products and 24 judged queries):

   ```bash
   make prep
   ```

   Peek at what it produced: `data/corpus.jsonl`, `data/queries.tsv`,
   `data/qrels.txt` (TREC format: `query_id 0 product_id grade`).

3. Start the service:

   ```bash
   make run
   ```

4. Open <http://localhost:8081>. The UI loads — logo, search box, strategy
   dropdown. Now search for anything.

## What you should see

An error: **`LAB 1: embedding provider not implemented`**. That's the
workshop in one screenshot: the HTTP layer, the UI, config loading — all
provided and working — while the search *internals* are yours to build.
This page comes alive one lab at a time.

## Checkpoint

Keep `make run` running, and in another terminal:

```bash
make verify LAB=0
```

Passes when the data is prepared and the service answers.

## One query to follow

Pick this query and re-run it in every lab: **`ergonomic chair`**. It's one
of the sample's 24 *judged* queries — human annotators graded which products
answer it — so you'll meet it again when the evaluation labs score your
work. Watching its results change — from error, to semantic matches, to
filtered, to hybrid — is the workshop's story arc.

Next: [Lab 1 — Local embeddings with a Redis cache](lab-1.md)

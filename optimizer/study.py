"""Lab 8: the Redis Retrieval Optimizer search study.

The Go service (cmd/searchd) built and loaded the index matrix; this
script measures it. For every (embedding model, index algorithm)
combination built by `make reindex-matrix`, it runs a set of named
search methods over the judged WANDS queries and scores them against
the qrels — producing one ranked table that answers Lab 8's question:
which configuration should go to production?

The study never rebuilds indexes or re-embeds the corpus: it connects
to the indexes the Go app already built (a deliberate polyglot moment —
build in Go, measure with the best available evaluation tool).

Usage:
    uv run python study.py

Environment:
    REDIS_URL          (default redis://localhost:6379)
    WORKSHOP_RUN_ID    (default local — must match the Go config)
"""

from __future__ import annotations

import json
import logging
import os
import warnings
from pathlib import Path

# Keep the study output readable in class: ranx ships LaTeX templates that
# trip SyntaxWarning on import, and redisvl marks its FT.HYBRID query
# classes experimental (we accept that — the Go side uses FT.HYBRID too).
warnings.filterwarnings("ignore", category=SyntaxWarning)
warnings.filterwarnings("ignore", message=".*experimental.*")

import pandas as pd
from ranx import Run
from redis import Redis
from redisvl.query import VectorQuery
from redisvl.query.filter import Num
from redisvl.query.hybrid import HybridQuery

from redis_retrieval_optimizer.schema import SearchMethodInput, SearchMethodOutput
from redis_retrieval_optimizer.search_methods import SEARCH_METHOD_MAP
from redis_retrieval_optimizer.search_methods.base import run_search_w_time
from redis_retrieval_optimizer.search_methods.vector import make_score_dict_vec
from redis_retrieval_optimizer.search_study import run_search_study

logging.basicConfig(level=logging.WARNING)

REDIS_URL = os.environ.get("REDIS_URL", "redis://localhost:6379")
RUN_ID = os.environ.get("WORKSHOP_RUN_ID", "local")

DATA_DIR = Path(__file__).parent.parent / "data"
WORK_DIR = Path(__file__).parent / ".study"

RET_K = 25

# The index matrix `make reindex-matrix` builds. Index names follow the Go
# app's convention: wands-{run_id}-{model_slug}-{algorithm}.
MODELS = [
    {"model": "sentence-transformers/all-MiniLM-L6-v2", "slug": "all-minilm-l6-v2",
     "dim": 384, "algorithms": ["flat", "hnsw", "svs-vamana"]},
    {"model": "sentence-transformers/all-mpnet-base-v2", "slug": "all-mpnet-base-v2",
     "dim": 768, "algorithms": ["flat", "hnsw"]},
]

# The strategy axis: text vs vector vs hybrid vs hybrid+filters, plus a
# linear-weight sweep. "bm25" and "vector" are optimizer built-ins; the
# rest are registered below via the search_method_map extension point.
SEARCH_METHODS = [
    "bm25",
    "vector",
    "vector_filtered",
    "hybrid_rrf",
    "hybrid_rrf_filtered",
    "hybrid_linear_text_020",
    "hybrid_linear_text_035",
    "hybrid_linear_text_050",
    "hybrid_linear_text_065",
    "hybrid_linear_text_080",
]

# Catalog constraint used by the *_filtered methods: well-reviewed
# products only. The same filter Lab 4 exposes in the API.
QUALITY_FILTER = (Num("average_rating") >= 4) & (Num("review_count") >= 1)


def convert_data() -> tuple[str, str]:
    """Convert the Go prep tool's outputs into the optimizer's formats.

    queries.tsv  -> {query_id: text}
    qrels.txt    -> {query_id: {product_id: grade}}   (TREC -> ranx dict)
    """
    WORK_DIR.mkdir(exist_ok=True)

    queries: dict[str, str] = {}
    with open(DATA_DIR / "queries.tsv", encoding="utf-8") as f:
        next(f)  # header
        for line in f:
            query_id, _, text = line.rstrip("\n").partition("\t")
            if query_id and text:
                queries[query_id] = text

    qrels: dict[str, dict[str, int]] = {}
    with open(DATA_DIR / "qrels.txt", encoding="utf-8") as f:
        for line in f:
            parts = line.split()
            if len(parts) != 4:
                continue
            query_id, _, product_id, grade = parts
            qrels.setdefault(query_id, {})[product_id] = int(grade)

    queries_path = WORK_DIR / "queries.json"
    qrels_path = WORK_DIR / "qrels.json"
    queries_path.write_text(json.dumps(queries), encoding="utf-8")
    qrels_path.write_text(json.dumps(qrels), encoding="utf-8")
    print(f"study data ready: {len(queries)} queries, {len(qrels)} judged")
    return str(queries_path), str(qrels_path)


def make_hybrid_method(combination_method: str = "LINEAR", text_weight: float = 0.5,
                       filtered: bool = False):
    """Build a parameterized FT.HYBRID search method.

    In Redis's linear fusion, alpha is the TEXT weight and 1 - alpha the
    vector weight, so text_weight=0.65 means 65% lexical, 35% semantic.
    """

    def gather(inputs: SearchMethodInput) -> SearchMethodOutput:
        results = {}
        for key, text in inputs.raw_queries.items():
            try:
                vector = inputs.emb_model.embed(
                    text, as_buffer=True, normalize_embeddings=True
                )
                kwargs = dict(
                    text=text,
                    text_field_name=inputs.text_field_name,
                    vector=vector,
                    vector_field_name=inputs.vector_field_name,
                    text_scorer="BM25STD",
                    combination_method=combination_method,
                    num_results=inputs.ret_k,
                    return_fields=[inputs.id_field_name, inputs.text_field_name],
                    yield_combined_score_as="hybrid_score",
                )
                if combination_method == "LINEAR":
                    kwargs["linear_alpha"] = text_weight
                else:
                    kwargs["rrf_window"] = max(inputs.ret_k, 20)
                    kwargs["rrf_constant"] = 60
                if filtered:
                    kwargs["filter_expression"] = QUALITY_FILTER

                res = run_search_w_time(
                    inputs.index, HybridQuery(**kwargs), inputs.query_metrics
                )
                scores = {}
                for rec in res:
                    if inputs.id_field_name in rec:
                        scores[rec[inputs.id_field_name]] = float(rec["hybrid_score"])
                results[key] = scores or {"no_match": 0}
            except Exception as e:  # noqa: BLE001 — a failed query scores zero
                logging.warning("hybrid query failed for %s: %s", key, e)
                results[key] = {"no_match": 0}
        return SearchMethodOutput(run=Run(results), query_metrics=inputs.query_metrics)

    return gather


def gather_vector_filtered(inputs: SearchMethodInput) -> SearchMethodOutput:
    """Vector KNN restricted to well-reviewed products (Lab 4's filter)."""
    results = {}
    for key, text in inputs.raw_queries.items():
        try:
            vector = inputs.emb_model.embed(
                text, as_buffer=True, normalize_embeddings=True
            )
            query = VectorQuery(
                vector=vector,
                vector_field_name=inputs.vector_field_name,
                num_results=inputs.ret_k,
                return_fields=[inputs.id_field_name, inputs.text_field_name],
                filter_expression=QUALITY_FILTER,
            )
            res = run_search_w_time(inputs.index, query, inputs.query_metrics)
            results[key] = make_score_dict_vec(res, inputs.id_field_name)
        except Exception as e:  # noqa: BLE001
            logging.warning("filtered vector query failed for %s: %s", key, e)
            results[key] = {"no_match": 0}
    return SearchMethodOutput(run=Run(results), query_metrics=inputs.query_metrics)


def build_search_method_map() -> dict:
    methods = dict(SEARCH_METHOD_MAP)  # keep the built-ins (bm25, vector, ...)
    methods["vector_filtered"] = gather_vector_filtered
    methods["hybrid_rrf"] = make_hybrid_method("RRF")
    methods["hybrid_rrf_filtered"] = make_hybrid_method("RRF", filtered=True)
    for alpha in (0.20, 0.35, 0.50, 0.65, 0.80):
        name = f"hybrid_linear_text_{int(alpha * 100):03d}"
        methods[name] = make_hybrid_method("LINEAR", text_weight=alpha)
    return methods


def index_exists(client: Redis, name: str) -> bool:
    try:
        client.ft(name).info()
        return True
    except Exception:  # noqa: BLE001 — "Unknown index name" or connection error
        return False


def main() -> None:
    queries_path, qrels_path = convert_data()
    method_map = build_search_method_map()
    client = Redis.from_url(REDIS_URL)

    frames = []
    for spec in MODELS:
        for algorithm in spec["algorithms"]:
            index_name = f"wands-{RUN_ID}-{spec['slug']}-{algorithm}"
            if not index_exists(client, index_name):
                print(f"skipping {index_name} (not built — run `make reindex-matrix`)")
                continue

            print(f"\n=== study: {index_name} ===")
            df = run_search_study(
                redis_url=REDIS_URL,
                config={
                    "study_id": f"wands-study-{RUN_ID}-{spec['slug']}-{algorithm}",
                    "index_name": index_name,
                    "queries": queries_path,
                    "qrels": qrels_path,
                    "search_methods": SEARCH_METHODS,
                    "ret_k": RET_K,
                    "id_field_name": "product_id",
                    "text_field_name": "search_text",
                    "vector_field_name": "embedding",
                    "embedding_model": {
                        "type": "hf",
                        "model": spec["model"],
                        "dim": spec["dim"],
                        "embedding_cache_name": f"optimizer-cache-{RUN_ID}",
                    },
                },
                search_method_map=method_map,
            )
            df.insert(0, "algorithm", algorithm)
            df.insert(0, "model", spec["slug"])
            frames.append(df)

    if not frames:
        raise SystemExit(
            "no indexes found — run `make reindex-matrix` first "
            f"(REDIS_URL={REDIS_URL}, WORKSHOP_RUN_ID={RUN_ID})"
        )

    table = pd.concat(frames, ignore_index=True)
    table = table.sort_values(
        by=["ndcg", "recall", "avg_query_time"],
        ascending=[False, False, True],
    ).reset_index(drop=True)

    columns = ["model", "algorithm", "search_method", "ndcg", "recall",
               "precision", "avg_query_time", "total_memory_mb"]
    print("\n=== combined study results (sorted by nDCG, then recall, then latency) ===\n")
    with pd.option_context("display.max_rows", None, "display.width", 160):
        print(table[columns].round(4).to_string())

    out = WORK_DIR / "study-results.csv"
    table.to_csv(out, index=False)
    best = table.iloc[0]
    print(f"\nresults saved to {out}")
    print(
        f"\nbest configuration: {best['model']} + {best['algorithm']} + "
        f"{best['search_method']} (nDCG {best['ndcg']:.4f})"
    )


if __name__ == "__main__":
    main()

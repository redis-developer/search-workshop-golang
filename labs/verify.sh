#!/usr/bin/env bash
# Checkpoint verification per lab: make verify LAB=<n>
# Requires the service running (make run) for labs >= 0.
set -u

BASE="${SEARCHD_URL:-http://localhost:8081}"
LAB="${1:-}"

pass() { echo "✅ LAB $LAB checkpoint passed: $1"; exit 0; }
fail() { echo "❌ LAB $LAB checkpoint failed: $1"; exit 1; }

json() { # json <url> <python-expr over data>
  curl -sf "$BASE$1" | python3 -c "
import json, sys
data = json.load(sys.stdin)
sys.exit(0 if ($2) else 1)
" 2>/dev/null
}

# A product ID that is guaranteed to exist: the first one in the corpus.
first_product_id() {
  head -1 data/corpus.jsonl | python3 -c 'import json,sys; print(json.load(sys.stdin)["product_id"])' 2>/dev/null
}

# "ergonomic chair" is one of the sample's judged queries, so every
# implemented strategy must return results for it.
QUERY="ergonomic%20chair"

case "$LAB" in
  0)
    [ -f data/corpus.jsonl ] || fail "data/corpus.jsonl missing; run 'make prep'"
    [ -f data/qrels.txt ]    || fail "data/qrels.txt missing; run 'make prep'"
    json /healthz "data.get('status') == 'ok'" \
      || fail "service not answering; run 'make run' in another terminal"
    pass "data prepared, Redis reachable, service up (searches unlock in Lab 1)"
    ;;
  1)
    json /healthz "data.get('status') == 'ok'" || fail "/healthz not ok; is 'make run' running?"
    # Lab 1 succeeds when startup got past embedding: cache keys exist.
    docker exec workshop-redis redis-cli --scan --pattern 'embedcache-*' 2>/dev/null | head -1 | grep -q . \
      || fail "no embedcache-* keys in Redis; embeddings not generated yet (restart 'make run' and watch for the embedding message)"
    pass "embeddings generated and cached in Redis"
    ;;
  2)
    want=$(wc -l < data/corpus.jsonl | tr -d ' ')
    json /stats "int(float(data.get('num_docs', 0))) == $want" \
      || fail "/stats does not report $want docs; index not loaded (restart 'make run')"
    pid=$(first_product_id)
    [ -n "$pid" ] || fail "could not read a product id from data/corpus.jsonl"
    json "/products/$pid" "'product_name' in data" || fail "/products/$pid not fetchable"
    pass "index created and $want products loaded"
    ;;
  3)
    json "/search?query=$QUERY&strategy=vector" \
      "len(data.get('matchedProducts', [])) > 0 and data['meta']['strategy'] == 'vector'" \
      || fail "vector search returned no results (restart 'make run' after pasting the code)"
    pass "vector search returns results"
    ;;
  4)
    json "/search?query=$QUERY&strategy=vector&min_rating=4" \
      "data['meta']['filtered'] and len(data['matchedProducts']) > 0 and all(p['rating'] >= 4 for p in data['matchedProducts'])" \
      || fail "filtered search missing, empty, or returned products below the rating floor"
    pass "filtered vector search enforces constraints"
    ;;
  5)
    json "/search?query=$QUERY&strategy=hybrid" \
      "len(data.get('matchedProducts', [])) > 0 and data['meta']['strategy'] == 'hybrid'" \
      || fail "hybrid search returned no results"
    json "/search?query=$QUERY&strategy=text" \
      "data['meta']['strategy'] == 'text'" || fail "text strategy not answering"
    pass "text and hybrid strategies both answer"
    ;;
  6)
    json "/facets" \
      "len(data.get('facets', [])) > 0 and data['facets'][0]['count'] >= data['facets'][-1]['count']" \
      || fail "/facets empty or not sorted by count"
    pass "facets aggregate the catalog by class"
    ;;
  7|8|9)
    echo "Labs 7-9 have no automated checkpoint; their output is the point."
    echo "Lab 7: make eval    Lab 8: make reindex-matrix && make study"
    exit 0
    ;;
  *)
    echo "usage: make verify LAB=<0-9>"
    exit 1
    ;;
esac

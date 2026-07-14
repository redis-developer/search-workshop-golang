#!/usr/bin/env bash
# Checkpoint verification per lab: make verify LAB=<n>
# Requires the service running (make run) for labs >= 1.
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

case "$LAB" in
  0)
    [ -f data/corpus.jsonl ] || fail "data/corpus.jsonl missing — run 'make prep'"
    [ -f data/qrels.txt ]    || fail "data/qrels.txt missing — run 'make prep'"
    curl -sf "$BASE/healthz" >/dev/null || fail "service not answering — run 'make run'"
    pass "data prepared, Redis reachable, service up"
    ;;
  1)
    json /healthz "data.get('status') == 'ok'" || fail "/healthz not ok"
    # Lab 1 succeeds when startup gets past embedding: cache keys exist.
    docker exec workshop-redis redis-cli --scan --pattern 'embedcache-*' 2>/dev/null | head -1 | grep -q . \
      || fail "no embedcache-* keys in Redis — embeddings not generated yet"
    pass "embeddings generated and cached in Redis"
    ;;
  2)
    json /stats "int(float(data.get('num_docs', 0))) == 600" \
      || fail "/stats does not report 600 docs — index not loaded"
    json /products/42 "'product_name' in data" || fail "/products/42 not fetchable"
    pass "index created and 600 products loaded"
    ;;
  3)
    json "/search?query=outdoor%20sofa&strategy=vector" \
      "len(data.get('matchedProducts', [])) > 0 and data['meta']['strategy'] == 'vector'" \
      || fail "vector search returned no results"
    pass "vector search returns results"
    ;;
  4)
    json "/search?query=outdoor%20sofa&strategy=vector&min_rating=4" \
      "data['meta']['filtered'] and all(p['rating'] >= 4 for p in data['matchedProducts'])" \
      || fail "filtered search missing or returned products below the rating floor"
    pass "filtered vector search enforces constraints"
    ;;
  5)
    json "/search?query=outdoor%20sofa&strategy=hybrid" \
      "len(data.get('matchedProducts', [])) > 0 and data['meta']['strategy'] == 'hybrid'" \
      || fail "hybrid search returned no results"
    json "/search?query=outdoor%20sofa&strategy=text" \
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
    echo "Labs 7-9 have no automated checkpoint — their output is the point."
    echo "Lab 7: make eval    Lab 8: make reindex-matrix && make study"
    exit 0
    ;;
  *)
    echo "usage: make verify LAB=<0-9>"
    exit 1
    ;;
esac

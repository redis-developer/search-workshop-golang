package catalog

import (
	"sort"
)

// Sample is the deterministic workshop slice of WANDS: a product corpus,
// the judged queries that exercise it, and the judgments restricted to
// both.
type Sample struct {
	Products  []Product
	Queries   []Query
	Judgments []Judgment
}

// BuildSample selects a deterministic, judgment-dense slice of WANDS
// sized for live workshop use.
//
// The sampler is query-first:
//
//  1. Queries are ranked by how many Exact judgments they have (ties
//     broken by query ID), and the top queryCount are kept; these are
//     the queries the evaluation labs can actually score.
//  2. The product budget is spent round-robin across the kept queries,
//     each query contributing its judged products best-grade-first. The
//     first pass therefore guarantees every kept query at least one
//     relevant product when the budget allows. Judged-Irrelevant
//     products are included too: they are realistic hard negatives.
//  3. Any remaining budget is filled with unjudged products in product-ID
//     order, so vector search has distractors beyond the judged pool.
//
// The same inputs always produce the same sample (no seeds, no RNG),
// which keeps every participant's metrics comparable.
func BuildSample(products []Product, queries []Query, judgments []Judgment, productCount, queryCount int) Sample {
	byQuery := make(map[string][]Judgment)
	exactCount := make(map[string]int)
	for _, j := range judgments {
		byQuery[j.QueryID] = append(byQuery[j.QueryID], j)
		if j.Grade == GradeExact {
			exactCount[j.QueryID]++
		}
	}

	// Rank judged queries: most Exact judgments first, query ID as the
	// deterministic tie-breaker.
	judgedQueries := make([]Query, 0, len(queries))
	for _, q := range queries {
		if len(byQuery[q.ID]) > 0 {
			judgedQueries = append(judgedQueries, q)
		}
	}
	sort.Slice(judgedQueries, func(a, b int) bool {
		qa, qb := judgedQueries[a], judgedQueries[b]
		if exactCount[qa.ID] != exactCount[qb.ID] {
			return exactCount[qa.ID] > exactCount[qb.ID]
		}
		return qa.ID < qb.ID
	})
	if queryCount > 0 && len(judgedQueries) > queryCount {
		judgedQueries = judgedQueries[:queryCount]
	}

	// Per-query judged products, best grade first (product ID tie-break).
	perQuery := make([][]Judgment, len(judgedQueries))
	for i, q := range judgedQueries {
		js := append([]Judgment(nil), byQuery[q.ID]...)
		sort.Slice(js, func(a, b int) bool {
			if js[a].Grade != js[b].Grade {
				return js[a].Grade > js[b].Grade
			}
			return js[a].ProductID < js[b].ProductID
		})
		perQuery[i] = js
	}

	productByID := make(map[string]Product, len(products))
	for _, p := range products {
		productByID[p.ID] = p
	}

	// Round-robin the product budget across queries.
	selected := make(map[string]bool)
	var selectedOrder []string
	take := func(id string) {
		if _, exists := productByID[id]; exists && !selected[id] {
			selected[id] = true
			selectedOrder = append(selectedOrder, id)
		}
	}
	cursors := make([]int, len(perQuery))
	for len(selectedOrder) < productCount {
		progressed := false
		for i := range perQuery {
			for cursors[i] < len(perQuery[i]) {
				j := perQuery[i][cursors[i]]
				cursors[i]++
				if !selected[j.ProductID] {
					take(j.ProductID)
					progressed = true
					break
				}
			}
			if len(selectedOrder) >= productCount {
				break
			}
		}
		if !progressed {
			break
		}
	}

	// Fill any remaining budget with unjudged distractors, product-ID
	// order for determinism.
	if len(selectedOrder) < productCount {
		rest := make([]Product, 0, len(products))
		for _, p := range products {
			if !selected[p.ID] {
				rest = append(rest, p)
			}
		}
		sort.Slice(rest, func(a, b int) bool { return rest[a].ID < rest[b].ID })
		for _, p := range rest {
			if len(selectedOrder) >= productCount {
				break
			}
			take(p.ID)
		}
	}

	sampleProducts := make([]Product, 0, len(selectedOrder))
	for _, id := range selectedOrder {
		sampleProducts = append(sampleProducts, productByID[id])
	}
	sort.Slice(sampleProducts, func(a, b int) bool { return sampleProducts[a].ID < sampleProducts[b].ID })

	// Restrict judgments to the sampled queries and products.
	keptQuery := make(map[string]bool, len(judgedQueries))
	for _, q := range judgedQueries {
		keptQuery[q.ID] = true
	}
	var sampleJudgments []Judgment
	for _, j := range judgments {
		if keptQuery[j.QueryID] && selected[j.ProductID] {
			sampleJudgments = append(sampleJudgments, j)
		}
	}
	sort.Slice(sampleJudgments, func(a, b int) bool {
		ja, jb := sampleJudgments[a], sampleJudgments[b]
		if ja.QueryID != jb.QueryID {
			return ja.QueryID < jb.QueryID
		}
		return ja.ProductID < jb.ProductID
	})

	return Sample{Products: sampleProducts, Queries: judgedQueries, Judgments: sampleJudgments}
}

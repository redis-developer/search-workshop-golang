// Command eval scores the current configuration against the WANDS
// relevance judgments: the instrument for Lab 7's tuning experiments.
//
// It runs every judged query through the same search code the API uses,
// then reports three numbers per configuration:
//
//   - nDCG@10: graded first-page relevance (the headline metric)
//   - Recall@25: coverage guardrail: of the known-relevant products,
//     how many made the top 25? WANDS queries often have MANY relevant
//     products, so read this as a floor, not a grade.
//   - avg query ms: measured client-side, per query
//
// Usage:
//
//	go run ./cmd/eval                    # score the config.yaml strategy
//	go run ./cmd/eval -strategy hybrid   # score a specific strategy
//	go run ./cmd/eval -v                 # include per-query rows
package main

import (
	"context"
	"flag"
	"fmt"
	"math"
	"os"
	"sort"
	"time"

	"github.com/redis-developer/search-workshop-golang/internal/catalog"
	"github.com/redis-developer/search-workshop-golang/internal/config"
	"github.com/redis-developer/search-workshop-golang/internal/search"
)

const (
	ndcgDepth   = 10
	recallDepth = 25
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to the workshop config file")
	strategy := flag.String("strategy", "", "override the search strategy (text|vector|hybrid)")
	verbose := flag.Bool("v", false, "print per-query metrics")
	flag.Parse()

	if err := run(*configPath, *strategy, *verbose); err != nil {
		fmt.Fprintln(os.Stderr, "eval:", err)
		os.Exit(1)
	}
}

func run(configPath, strategy string, verbose bool) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}
	if strategy == "" {
		strategy = cfg.Search.DefaultStrategy
	}

	queries, err := catalog.ReadQueries("data/queries.tsv")
	if err != nil {
		return fmt.Errorf("reading queries (run `make prep` first): %w", err)
	}
	judgments, err := catalog.ReadQrels("data/qrels.txt")
	if err != nil {
		return err
	}
	grades := gradeMap(judgments)

	ctx := context.Background()
	svc, err := search.New(ctx, cfg)
	if err != nil {
		return err
	}
	defer svc.Close() //nolint:errcheck // shutdown path
	if err := svc.EnsureIndex(ctx, false); err != nil {
		return err
	}

	fmt.Printf("scoring %q on %s (%d judged queries)\n\n",
		strategy, cfg.IndexName(), len(queries))
	if verbose {
		fmt.Printf("%-28s  %8s  %9s  %8s\n", "query", "nDCG@10", "Recall@25", "ms")
	}

	var sumNDCG, sumRecall, sumMS float64
	for _, q := range queries {
		start := time.Now()
		resp, err := svc.Search(ctx, search.Request{
			Query:    q.Text,
			Strategy: strategy,
			K:        recallDepth,
		})
		if err != nil {
			return fmt.Errorf("query %q: %w", q.Text, err)
		}
		ms := float64(time.Since(start).Microseconds()) / 1000

		ranked := resp.ProductIDs()
		n := ndcg(ranked, grades[q.ID], ndcgDepth)
		r := recall(ranked, grades[q.ID], recallDepth)
		sumNDCG += n
		sumRecall += r
		sumMS += ms
		if verbose {
			fmt.Printf("%-28.28s  %8.4f  %9.4f  %8.2f\n", q.Text, n, r, ms)
		}
	}

	count := float64(len(queries))
	if verbose {
		fmt.Println()
	}
	fmt.Printf("configuration:  %s\n", cfg.Describe(strategy))
	fmt.Printf("nDCG@10:        %.4f\n", sumNDCG/count)
	fmt.Printf("Recall@25:      %.4f\n", sumRecall/count)
	fmt.Printf("avg query time: %.2f ms\n", sumMS/count)
	return nil
}

// gradeMap indexes judgments by query, then by product.
func gradeMap(judgments []catalog.Judgment) map[string]map[string]int {
	grades := make(map[string]map[string]int)
	for _, j := range judgments {
		if grades[j.QueryID] == nil {
			grades[j.QueryID] = make(map[string]int)
		}
		grades[j.QueryID][j.ProductID] = j.Grade
	}
	return grades
}

// ndcg computes normalized discounted cumulative gain at depth k with the
// standard log2 rank discount. Unjudged products count as grade 0, the
// conventional (conservative) assumption.
func ndcg(ranked []string, grades map[string]int, k int) float64 {
	if len(ranked) > k {
		ranked = ranked[:k]
	}
	var dcg float64
	for i, id := range ranked {
		gain := float64(grades[id])
		dcg += gain / math.Log2(float64(i)+2)
	}

	// Ideal DCG: the best possible ordering of this query's judged grades.
	ideal := make([]int, 0, len(grades))
	for _, g := range grades {
		ideal = append(ideal, g)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(ideal)))
	var idcg float64
	for i, g := range ideal {
		if i >= k {
			break
		}
		idcg += float64(g) / math.Log2(float64(i)+2)
	}
	if idcg == 0 {
		return 0
	}
	return dcg / idcg
}

// recall computes Recall@k over binary relevance (grade > 0). The
// denominator is capped at k so queries with more relevant products than
// k do not report an unreachable ceiling.
func recall(ranked []string, grades map[string]int, k int) float64 {
	if len(ranked) > k {
		ranked = ranked[:k]
	}
	totalRelevant := 0
	for _, g := range grades {
		if g > 0 {
			totalRelevant++
		}
	}
	if totalRelevant == 0 {
		return 0
	}
	if totalRelevant > k {
		totalRelevant = k
	}
	found := 0
	for _, id := range ranked {
		if grades[id] > 0 {
			found++
		}
	}
	return float64(found) / float64(totalRelevant)
}

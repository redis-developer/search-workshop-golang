package search

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis-developer/redis-vl-golang/filter"
	"github.com/redis-developer/redis-vl-golang/query"

	"github.com/redis-developer/search-workshop-golang/internal/config"
)

// Request is one search call: the user's query text plus optional
// strategy override and catalog filters.
type Request struct {
	Query string
	// Strategy is text | vector | hybrid; empty uses the config default.
	Strategy string
	// Class filters on product_class (exact tag match).
	Class string
	// MinRating filters on average_rating >= MinRating.
	MinRating float64
	// MinReviews filters on review_count >= MinReviews.
	MinReviews int
	// K overrides the configured result count when > 0.
	K int
}

// Item is one search hit, shaped for the UI's results table.
type Item struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Class       string  `json:"class"`
	Description string  `json:"description"`
	Features    string  `json:"features"`
	Rating      float64 `json:"rating"`
	// Score is strategy-dependent: BM25 score for text, cosine distance
	// for vector (lower is closer), fused score for hybrid.
	Score float64 `json:"score"`
}

// Meta describes which configuration answered a search: the workshop's
// observability surface.
type Meta struct {
	Strategy       string  `json:"strategy"`
	Fusion         string  `json:"fusion,omitempty"`
	Alpha          float64 `json:"alpha,omitempty"`
	IndexAlgorithm string  `json:"index_algorithm"`
	EmbeddingModel string  `json:"embedding_model"`
	K              int     `json:"k"`
	Filtered       bool    `json:"filtered"`
	QueryMS        float64 `json:"query_ms"`
}

// Response is a completed search.
type Response struct {
	// ResultType is the compact configuration string the UI displays
	// next to the elapsed time.
	ResultType string `json:"resultType"`
	Items      []Item `json:"matchedProducts"`
	Meta       Meta   `json:"meta"`
}

// returnFields are fetched for every strategy so the UI table can render.
var returnFields = []string{
	FieldName, FieldClass, FieldDescription, FieldFeatures, FieldAverageRating,
}

// Search runs one search with the requested (or configured) strategy.
// This dispatcher is filled in across Labs 3-5: vector first, then
// filters, then text and hybrid.
func (s *Service) Search(ctx context.Context, req Request) (*Response, error) {
	strategy := req.Strategy
	if strategy == "" {
		strategy = s.cfg.Search.DefaultStrategy
	}
	k := req.K
	if k <= 0 {
		k = s.cfg.Search.K
	}
	f := buildFilter(req)

	start := time.Now()
	var (
		rows []map[string]any
		err  error
	)
	switch strategy {
	case config.StrategyText:
		rows, err = s.searchText(ctx, req.Query, f, k)
	case config.StrategyVector:
		rows, err = s.searchVector(ctx, req.Query, f, k)
	case config.StrategyHybrid:
		rows, err = s.searchHybrid(ctx, req.Query, f, k)
	default:
		return nil, fmt.Errorf("unknown strategy %q (want text, vector, or hybrid)", strategy)
	}
	if err != nil {
		return nil, err
	}
	elapsed := float64(time.Since(start).Microseconds()) / 1000

	items := make([]Item, 0, len(rows))
	for _, row := range rows {
		items = append(items, itemFromRow(row, s.index.Prefix()))
	}

	meta := Meta{
		Strategy:       strategy,
		IndexAlgorithm: s.cfg.Index.Algorithm,
		EmbeddingModel: s.vec.ModelName(),
		K:              k,
		Filtered:       f != nil,
		QueryMS:        elapsed,
	}
	if strategy == config.StrategyHybrid {
		meta.Fusion = s.cfg.Search.Hybrid.Fusion
		if meta.Fusion == config.FusionLinear {
			meta.Alpha = s.cfg.Search.Hybrid.Alpha
		}
	}
	return &Response{
		ResultType: s.cfg.Describe(strategy),
		Items:      items,
		Meta:       meta,
	}, nil
}

// searchText is plain lexical search: BM25 over search_text (Lab 5
// introduces it as the hybrid baseline).
func (s *Service) searchText(ctx context.Context, text string, f *filter.Expression, k int) ([]map[string]any, error) {
	q := query.NewTextQuery(text, FieldSearchText).
		NumResults(k).
		ReturnFields(returnFields...)
	if f != nil {
		q.Filter(f)
	}
	return s.index.Query(ctx, q)
}

// searchVector is semantic KNN search: embed the query (served from the
// embeddings cache when repeated), then find the nearest products
// (Lab 3; Lab 4 adds the filter).
func (s *Service) searchVector(ctx context.Context, text string, f *filter.Expression, k int) ([]map[string]any, error) {
	// LAB 3 (reference solution): embed the query text, then run KNN.
	vec, err := s.vec.Embed(ctx, text)
	if err != nil {
		return nil, fmt.Errorf("embedding query: %w", err)
	}
	q := query.NewVectorQuery(FieldEmbedding, vec).
		NumResults(k).
		ReturnFields(returnFields...)
	if f != nil {
		// LAB 4 (reference solution): pre-filter the KNN candidates.
		q.Filter(f)
	}
	return s.index.Query(ctx, q)
}

// searchHybrid fuses a lexical leg and a semantic leg server-side with
// FT.HYBRID (Lab 5). Fusion method and weights come from config.yaml.
func (s *Service) searchHybrid(ctx context.Context, text string, f *filter.Expression, k int) ([]map[string]any, error) {
	vec, err := s.vec.Embed(ctx, text)
	if err != nil {
		return nil, fmt.Errorf("embedding query: %w", err)
	}
	q := query.NewHybridQuery(text, FieldSearchText, vec, FieldEmbedding).
		NumResults(k).
		ReturnFields(returnFields...)
	if f != nil {
		q.Filter(f)
	}
	switch s.cfg.Search.Hybrid.Fusion {
	case config.FusionLinear:
		q.CombineLinear(s.cfg.Search.Hybrid.Alpha)
	default:
		q.CombineRRF(s.cfg.Search.Hybrid.RRFWindow, s.cfg.Search.Hybrid.RRFConstant)
	}
	return s.index.Hybrid(ctx, q)
}

// buildFilter combines the request's catalog constraints into one filter
// expression (Lab 4). Returns nil when unfiltered.
func buildFilter(req Request) *filter.Expression {
	var exprs []*filter.Expression
	if req.Class != "" {
		exprs = append(exprs, filter.Tag(FieldClass).Eq(req.Class))
	}
	if req.MinRating > 0 {
		exprs = append(exprs, filter.Num(FieldAverageRating).Ge(req.MinRating))
	}
	if req.MinReviews > 0 {
		exprs = append(exprs, filter.Num(FieldReviewCount).Ge(float64(req.MinReviews)))
	}
	if len(exprs) == 0 {
		return nil
	}
	out := exprs[0]
	if len(exprs) > 1 {
		out = out.And(exprs[1:]...)
	}
	return out
}

// Facet is one product-class bucket with its document count and average
// rating.
type Facet struct {
	Class     string  `json:"class"`
	Count     int64   `json:"count"`
	AvgRating float64 `json:"avg_rating"`
}

// facetQuery is a minimal FT.AGGREGATE builder: group products by class,
// count them, and average their ratings (Lab 6).
type facetQuery struct{ limit int }

// AggregateArgs implements query.AggregationQuery.
func (q facetQuery) AggregateArgs(indexName string) []any {
	return []any{
		"FT.AGGREGATE", indexName, "*",
		"GROUPBY", 1, "@" + FieldClass,
		"REDUCE", "COUNT", 0, "AS", "count",
		"REDUCE", "AVG", 1, "@" + FieldAverageRating, "AS", "avg_rating",
		"SORTBY", 2, "@count", "DESC",
		"LIMIT", 0, q.limit,
		"DIALECT", 2,
	}
}

// Facets aggregates the catalog by product class.
func (s *Service) Facets(ctx context.Context, limit int) ([]Facet, error) {
	if limit <= 0 {
		limit = 25
	}
	rows, err := s.index.Aggregate(ctx, facetQuery{limit: limit})
	if err != nil {
		return nil, fmt.Errorf("aggregating facets: %w", err)
	}
	facets := make([]Facet, 0, len(rows))
	for _, row := range rows {
		facets = append(facets, Facet{
			Class:     asString(row[FieldClass]),
			Count:     int64(asFloat(row["count"])),
			AvgRating: asFloat(row["avg_rating"]),
		})
	}
	return facets, nil
}

// itemFromRow maps a raw result row to a UI item. FT.SEARCH-based
// queries report the document key as "id"; FT.HYBRID reports it as
// "__key". Either way the key is "<prefix>:<product_id>". The
// strategy-dependent score lives under different names per query type.
func itemFromRow(row map[string]any, prefix string) Item {
	id := asString(row["id"])
	if id == "" {
		id = asString(row["__key"])
	}
	id = strings.TrimPrefix(id, prefix+":")

	score := 0.0
	for _, key := range []string{"vector_distance", "hybrid_score", "combined_score", "__score", "score"} {
		if v, ok := row[key]; ok {
			score = asFloat(v)
			break
		}
	}
	return Item{
		ID:          id,
		Name:        asString(row[FieldName]),
		Class:       asString(row[FieldClass]),
		Description: asString(row[FieldDescription]),
		Features:    asString(row[FieldFeatures]),
		Rating:      asFloat(row[FieldAverageRating]),
		Score:       score,
	}
}

// ProductIDs extracts the bare product IDs from a response, in rank
// order, used by cmd/eval to score runs against the qrels.
func (r *Response) ProductIDs() []string {
	ids := make([]string, len(r.Items))
	for i, item := range r.Items {
		ids[i] = item.ID
	}
	return ids
}

func asString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case []byte:
		return string(x)
	case nil:
		return ""
	default:
		return fmt.Sprint(x)
	}
}

func asFloat(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int64:
		return float64(x)
	case string:
		f, _ := strconv.ParseFloat(x, 64)
		return f
	default:
		return 0
	}
}

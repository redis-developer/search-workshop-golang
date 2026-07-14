// Package catalog models the WANDS product-search dataset: products
// (the corpus), user queries, and graded relevance judgments (qrels).
//
// WANDS is the Wayfair ANnotation Dataset for product-search relevance
// assessment: https://github.com/wayfair/WANDS. The source files use a
// .csv extension but are tab-separated.
package catalog

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// Relevance grades mapped from the WANDS label column.
const (
	GradeExact      = 2 // "Exact" — the product answers the query
	GradePartial    = 1 // "Partial" — related but not a direct answer
	GradeIrrelevant = 0 // "Irrelevant" — judged and not relevant
)

// Product is one WANDS product record plus the derived search_text field.
type Product struct {
	ID            string  `json:"product_id"`
	Name          string  `json:"product_name"`
	Class         string  `json:"product_class"`
	Hierarchy     string  `json:"category_hierarchy"`
	Description   string  `json:"product_description"`
	Features      string  `json:"product_features"`
	AverageRating float64 `json:"average_rating"`
	RatingCount   int     `json:"rating_count"`
	ReviewCount   int     `json:"review_count"`
	// SearchText combines name, class, hierarchy, description, and
	// features into the single text field the index searches and embeds.
	SearchText string `json:"search_text"`
}

// Query is one WANDS user search query.
type Query struct {
	ID   string
	Text string
}

// Judgment is one graded relevance label: how relevant a product is to a
// query, on the Exact/Partial/Irrelevant scale.
type Judgment struct {
	QueryID   string
	ProductID string
	Grade     int
}

// BuildSearchText assembles the text used for lexical search and for
// embeddings. Feature entries arrive as "name:value|name:value"; the
// separators carry no meaning for search, so they become spaces.
func BuildSearchText(p Product) string {
	features := strings.NewReplacer("|", " ", ":", " ").Replace(p.Features)
	hierarchy := strings.ReplaceAll(p.Hierarchy, "/", " ")

	parts := make([]string, 0, 5)
	for _, s := range []string{p.Name, p.Class, hierarchy, p.Description, features} {
		if s = strings.TrimSpace(s); s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, ". ")
}

// tsvReader configures a csv.Reader for the WANDS tab-separated files.
func tsvReader(r io.Reader) *csv.Reader {
	cr := csv.NewReader(r)
	cr.Comma = '\t'
	cr.LazyQuotes = true
	cr.FieldsPerRecord = -1
	return cr
}

// headerIndex maps column names to positions, so parsing survives column
// reordering in the source files.
func headerIndex(header []string) map[string]int {
	idx := make(map[string]int, len(header))
	for i, name := range header {
		idx[strings.TrimSpace(strings.ToLower(name))] = i
	}
	return idx
}

func field(record []string, idx map[string]int, name string) string {
	i, ok := idx[name]
	if !ok || i >= len(record) {
		return ""
	}
	return strings.TrimSpace(record[i])
}

// ParseProducts reads the WANDS product.csv (tab-separated) file.
func ParseProducts(r io.Reader) ([]Product, error) {
	cr := tsvReader(r)
	header, err := cr.Read()
	if err != nil {
		return nil, fmt.Errorf("reading product header: %w", err)
	}
	idx := headerIndex(header)

	var products []Product
	for {
		record, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading product record: %w", err)
		}
		p := Product{
			ID:          field(record, idx, "product_id"),
			Name:        field(record, idx, "product_name"),
			Class:       field(record, idx, "product_class"),
			Hierarchy:   field(record, idx, "category hierarchy"),
			Description: field(record, idx, "product_description"),
			Features:    field(record, idx, "product_features"),
		}
		if p.ID == "" {
			continue
		}
		p.AverageRating, _ = strconv.ParseFloat(field(record, idx, "average_rating"), 64)
		p.RatingCount, _ = strconv.Atoi(field(record, idx, "rating_count"))
		p.ReviewCount, _ = strconv.Atoi(field(record, idx, "review_count"))
		p.SearchText = BuildSearchText(p)
		products = append(products, p)
	}
	return products, nil
}

// ParseQueries reads the WANDS query.csv (tab-separated) file.
func ParseQueries(r io.Reader) ([]Query, error) {
	cr := tsvReader(r)
	header, err := cr.Read()
	if err != nil {
		return nil, fmt.Errorf("reading query header: %w", err)
	}
	idx := headerIndex(header)

	var queries []Query
	for {
		record, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading query record: %w", err)
		}
		q := Query{
			ID:   field(record, idx, "query_id"),
			Text: field(record, idx, "query"),
		}
		if q.ID == "" || q.Text == "" {
			continue
		}
		queries = append(queries, q)
	}
	return queries, nil
}

// ParseJudgments reads the WANDS label.csv (tab-separated) file, mapping
// Exact/Partial/Irrelevant to numeric grades.
func ParseJudgments(r io.Reader) ([]Judgment, error) {
	cr := tsvReader(r)
	header, err := cr.Read()
	if err != nil {
		return nil, fmt.Errorf("reading label header: %w", err)
	}
	idx := headerIndex(header)

	var judgments []Judgment
	for {
		record, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading label record: %w", err)
		}
		j := Judgment{
			QueryID:   field(record, idx, "query_id"),
			ProductID: field(record, idx, "product_id"),
		}
		if j.QueryID == "" || j.ProductID == "" {
			continue
		}
		switch strings.ToLower(field(record, idx, "label")) {
		case "exact":
			j.Grade = GradeExact
		case "partial":
			j.Grade = GradePartial
		case "irrelevant":
			j.Grade = GradeIrrelevant
		default:
			continue
		}
		judgments = append(judgments, j)
	}
	return judgments, nil
}

// WriteCorpus writes products as JSON Lines.
func WriteCorpus(path string, products []Product) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close() //nolint:errcheck // error captured by the flush below

	w := bufio.NewWriter(f)
	enc := json.NewEncoder(w)
	for _, p := range products {
		if err := enc.Encode(p); err != nil {
			return fmt.Errorf("encoding product %s: %w", p.ID, err)
		}
	}
	if err := w.Flush(); err != nil {
		return err
	}
	return f.Close()
}

// ReadCorpus reads products from a JSON Lines file.
func ReadCorpus(path string) ([]Product, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close() //nolint:errcheck // read-only

	var products []Product
	dec := json.NewDecoder(bufio.NewReader(f))
	for dec.More() {
		var p Product
		if err := dec.Decode(&p); err != nil {
			return nil, fmt.Errorf("decoding corpus: %w", err)
		}
		products = append(products, p)
	}
	return products, nil
}

// WriteQueries writes queries as a two-column TSV (query_id, query).
func WriteQueries(path string, queries []Query) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close() //nolint:errcheck // error captured by the flush below

	w := bufio.NewWriter(f)
	fmt.Fprintln(w, "query_id\tquery")
	for _, q := range queries {
		fmt.Fprintf(w, "%s\t%s\n", q.ID, q.Text)
	}
	if err := w.Flush(); err != nil {
		return err
	}
	return f.Close()
}

// ReadQueries reads queries from the two-column TSV written by WriteQueries.
func ReadQueries(path string) ([]Query, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close() //nolint:errcheck // read-only
	return ParseQueries(f)
}

// WriteQrels writes judgments in TREC qrels format:
//
//	query_id 0 product_id grade
//
// This format is read by both cmd/eval (Go) and the Lab 8 Python
// optimizer study.
func WriteQrels(path string, judgments []Judgment) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close() //nolint:errcheck // error captured by the flush below

	w := bufio.NewWriter(f)
	for _, j := range judgments {
		fmt.Fprintf(w, "%s 0 %s %d\n", j.QueryID, j.ProductID, j.Grade)
	}
	if err := w.Flush(); err != nil {
		return err
	}
	return f.Close()
}

// ReadQrels reads a TREC-format qrels file.
func ReadQrels(path string) ([]Judgment, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close() //nolint:errcheck // read-only

	var judgments []Judgment
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) != 4 {
			return nil, fmt.Errorf("malformed qrels line: %q", line)
		}
		grade, err := strconv.Atoi(parts[3])
		if err != nil {
			return nil, fmt.Errorf("malformed qrels grade in %q: %w", line, err)
		}
		judgments = append(judgments, Judgment{QueryID: parts[0], ProductID: parts[2], Grade: grade})
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return judgments, nil
}

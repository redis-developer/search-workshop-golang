package catalog

import (
	"strings"
	"testing"
)

const productsTSV = "product_id\tproduct_name\tproduct_class\tcategory hierarchy\tproduct_description\tproduct_features\trating_count\taverage_rating\treview_count\n" +
	"1\tsolid wood platform bed\tBeds\tFurniture/Bedroom Furniture/Beds\ta sturdy bed frame\tcolor:brown|material:wood\t10\t4.5\t8\n" +
	"2\tvelvet loveseat\tSofas\tFurniture/Living Room Furniture/Sofas\tsoft two-seater\tcolor:green\t3\t3.9\t2\n" +
	"3\toutdoor patio sofa\tOutdoor Sofas\tOutdoor/Patio Furniture/Sofas\tweather resistant\t\t\t\t\n"

const queriesTSV = "query_id\tquery\tquery_class\n" +
	"q1\tplatform bed\tBeds\n" +
	"q2\toutdoor couch\tOutdoor Sofas\n" +
	"q3\tnever judged\tMisc\n"

const labelsTSV = "id\tquery_id\tproduct_id\tlabel\n" +
	"1\tq1\t1\tExact\n" +
	"2\tq1\t2\tIrrelevant\n" +
	"3\tq2\t3\tExact\n" +
	"4\tq2\t2\tPartial\n"

func TestParseProducts(t *testing.T) {
	products, err := ParseProducts(strings.NewReader(productsTSV))
	if err != nil {
		t.Fatalf("ParseProducts: %v", err)
	}
	if len(products) != 3 {
		t.Fatalf("got %d products, want 3", len(products))
	}
	p := products[0]
	if p.ID != "1" || p.Class != "Beds" || p.AverageRating != 4.5 || p.ReviewCount != 8 {
		t.Errorf("unexpected first product: %+v", p)
	}
	if p.Hierarchy != "Furniture/Bedroom Furniture/Beds" {
		t.Errorf("hierarchy = %q", p.Hierarchy)
	}
	for _, want := range []string{"solid wood platform bed", "Beds", "Furniture Bedroom Furniture Beds", "sturdy bed frame", "color brown", "material wood"} {
		if !strings.Contains(p.SearchText, want) {
			t.Errorf("search_text missing %q: %q", want, p.SearchText)
		}
	}
	// Missing numerics default to zero.
	if products[2].AverageRating != 0 || products[2].RatingCount != 0 {
		t.Errorf("empty numerics should parse as zero: %+v", products[2])
	}
}

func TestParseQueriesAndJudgments(t *testing.T) {
	queries, err := ParseQueries(strings.NewReader(queriesTSV))
	if err != nil {
		t.Fatalf("ParseQueries: %v", err)
	}
	if len(queries) != 3 || queries[1].Text != "outdoor couch" {
		t.Fatalf("unexpected queries: %+v", queries)
	}

	judgments, err := ParseJudgments(strings.NewReader(labelsTSV))
	if err != nil {
		t.Fatalf("ParseJudgments: %v", err)
	}
	if len(judgments) != 4 {
		t.Fatalf("got %d judgments, want 4", len(judgments))
	}
	grades := map[string]int{}
	for _, j := range judgments {
		grades[j.QueryID+"/"+j.ProductID] = j.Grade
	}
	if grades["q1/1"] != GradeExact || grades["q1/2"] != GradeIrrelevant || grades["q2/2"] != GradePartial {
		t.Errorf("unexpected grade mapping: %v", grades)
	}
}

func TestBuildSample(t *testing.T) {
	products, _ := ParseProducts(strings.NewReader(productsTSV))
	queries, _ := ParseQueries(strings.NewReader(queriesTSV))
	judgments, _ := ParseJudgments(strings.NewReader(labelsTSV))

	sample := BuildSample(products, queries, judgments, 2, 2)

	if len(sample.Queries) != 2 {
		t.Fatalf("got %d queries, want 2", len(sample.Queries))
	}
	for _, q := range sample.Queries {
		if q.ID == "q3" {
			t.Error("unjudged query q3 must not be sampled")
		}
	}
	if len(sample.Products) != 2 {
		t.Fatalf("got %d products, want 2", len(sample.Products))
	}

	// Every kept query keeps at least one relevant product (round-robin
	// first pass takes each query's best judgment).
	inSample := map[string]bool{}
	for _, p := range sample.Products {
		inSample[p.ID] = true
	}
	relevant := map[string]bool{}
	for _, j := range sample.Judgments {
		if j.Grade > 0 && inSample[j.ProductID] {
			relevant[j.QueryID] = true
		}
	}
	for _, q := range sample.Queries {
		if !relevant[q.ID] {
			t.Errorf("query %s has no relevant product in the sample", q.ID)
		}
	}

	// Qrels are restricted to sampled products and queries.
	for _, j := range sample.Judgments {
		if !inSample[j.ProductID] {
			t.Errorf("judgment references unsampled product %s", j.ProductID)
		}
	}

	// Determinism: same inputs, same sample.
	again := BuildSample(products, queries, judgments, 2, 2)
	if len(again.Products) != len(sample.Products) || len(again.Judgments) != len(sample.Judgments) {
		t.Fatal("BuildSample is not deterministic")
	}
	for i := range sample.Products {
		if sample.Products[i].ID != again.Products[i].ID {
			t.Fatal("BuildSample product order is not deterministic")
		}
	}
}

func TestSampleRoundTrip(t *testing.T) {
	dir := t.TempDir()
	products, _ := ParseProducts(strings.NewReader(productsTSV))
	queries, _ := ParseQueries(strings.NewReader(queriesTSV))
	judgments, _ := ParseJudgments(strings.NewReader(labelsTSV))

	corpusPath := dir + "/corpus.jsonl"
	queriesPath := dir + "/queries.tsv"
	qrelsPath := dir + "/qrels.txt"

	if err := WriteCorpus(corpusPath, products); err != nil {
		t.Fatalf("WriteCorpus: %v", err)
	}
	if err := WriteQueries(queriesPath, queries); err != nil {
		t.Fatalf("WriteQueries: %v", err)
	}
	if err := WriteQrels(qrelsPath, judgments); err != nil {
		t.Fatalf("WriteQrels: %v", err)
	}

	gotProducts, err := ReadCorpus(corpusPath)
	if err != nil {
		t.Fatalf("ReadCorpus: %v", err)
	}
	if len(gotProducts) != len(products) || gotProducts[0].SearchText != products[0].SearchText {
		t.Error("corpus round-trip mismatch")
	}
	gotQueries, err := ReadQueries(queriesPath)
	if err != nil {
		t.Fatalf("ReadQueries: %v", err)
	}
	if len(gotQueries) != len(queries) {
		t.Error("queries round-trip mismatch")
	}
	gotJudgments, err := ReadQrels(qrelsPath)
	if err != nil {
		t.Fatalf("ReadQrels: %v", err)
	}
	if len(gotJudgments) != len(judgments) || gotJudgments[0] != judgments[0] {
		t.Error("qrels round-trip mismatch")
	}
}

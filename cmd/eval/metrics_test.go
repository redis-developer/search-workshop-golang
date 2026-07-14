package main

import (
	"math"
	"testing"
)

func almostEqual(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestNDCGPerfectRanking(t *testing.T) {
	grades := map[string]int{"a": 2, "b": 1, "c": 0}
	if got := ndcg([]string{"a", "b", "c"}, grades, 10); !almostEqual(got, 1.0) {
		t.Errorf("perfect ranking nDCG = %v, want 1.0", got)
	}
}

func TestNDCGWorstRanking(t *testing.T) {
	grades := map[string]int{"a": 2, "b": 1}
	// Ranking only irrelevant/unjudged products scores zero.
	if got := ndcg([]string{"x", "y", "z"}, grades, 10); !almostEqual(got, 0.0) {
		t.Errorf("irrelevant ranking nDCG = %v, want 0", got)
	}
}

func TestNDCGSwappedRanking(t *testing.T) {
	grades := map[string]int{"a": 2, "b": 1}
	// b before a: DCG = 1/log2(2) + 2/log2(3); IDCG = 2/log2(2) + 1/log2(3)
	want := (1.0/math.Log2(2) + 2.0/math.Log2(3)) / (2.0/math.Log2(2) + 1.0/math.Log2(3))
	if got := ndcg([]string{"b", "a"}, grades, 10); !almostEqual(got, want) {
		t.Errorf("swapped ranking nDCG = %v, want %v", got, want)
	}
}

func TestNDCGDepthCutoff(t *testing.T) {
	grades := map[string]int{"a": 2}
	// The relevant product sits below the cutoff: no credit.
	ranked := []string{"x1", "x2", "x3", "a"}
	if got := ndcg(ranked, grades, 3); !almostEqual(got, 0.0) {
		t.Errorf("below-cutoff nDCG = %v, want 0", got)
	}
}

func TestRecall(t *testing.T) {
	grades := map[string]int{"a": 2, "b": 1, "c": 0, "d": 1}
	// 3 relevant total; 2 found in the top-k.
	if got := recall([]string{"a", "x", "b"}, grades, 25); !almostEqual(got, 2.0/3.0) {
		t.Errorf("recall = %v, want 2/3", got)
	}
}

func TestRecallDenominatorCappedAtK(t *testing.T) {
	// 5 relevant products but k=3: finding 3 is a full score.
	grades := map[string]int{"a": 1, "b": 1, "c": 1, "d": 1, "e": 1}
	if got := recall([]string{"a", "b", "c"}, grades, 3); !almostEqual(got, 1.0) {
		t.Errorf("capped recall = %v, want 1.0", got)
	}
}

func TestRecallNoRelevant(t *testing.T) {
	if got := recall([]string{"a"}, map[string]int{"a": 0}, 25); !almostEqual(got, 0.0) {
		t.Errorf("no-relevant recall = %v, want 0", got)
	}
}

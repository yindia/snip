package search

import "testing"

func TestNormalizeScores(t *testing.T) {
	results := []Result{{Score: 2}, {Score: 4}, {Score: 6}}
	normalizeScores(results)
	if results[0].Score != 0 || results[1].Score != 0.5 || results[2].Score != 1 {
		t.Fatalf("unexpected normalized scores: %#v", results)
	}
}

func TestApplyMinScore(t *testing.T) {
	results := []Result{{Score: 0.2}, {Score: 0.5}, {Score: 0.8}}
	filtered := applyMinScore(results, 0.5)
	if len(filtered) != 2 {
		t.Fatalf("expected 2 results, got %d", len(filtered))
	}
	if filtered[0].Score != 0.5 || filtered[1].Score != 0.8 {
		t.Fatalf("unexpected filtered scores: %#v", filtered)
	}
}

func TestNormalizeFloatScores(t *testing.T) {
	scores := []float64{2, 4, 6}
	normalized := normalizeFloatScores(scores)
	if normalized[0] != 0 || normalized[1] != 0.5 || normalized[2] != 1 {
		t.Fatalf("unexpected normalized scores: %#v", normalized)
	}
}

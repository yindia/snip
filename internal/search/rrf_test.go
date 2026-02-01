package search

import "testing"

func TestRRFFuse(t *testing.T) {
	list1 := rrfList{items: []Result{{DocID: "a"}, {DocID: "b"}}, weight: 1}
	list2 := rrfList{items: []Result{{DocID: "b"}, {DocID: "c"}}, weight: 1}
	out := rrfFuse([]rrfList{list1, list2})
	if len(out) != 3 {
		t.Fatalf("expected 3 results, got %d", len(out))
	}
	if out[0].DocID != "b" {
		t.Fatalf("expected doc b first, got %s", out[0].DocID)
	}
}

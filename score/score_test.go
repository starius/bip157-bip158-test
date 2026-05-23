package score

import "testing"

func TestSummarizeGreen(t *testing.T) {
	summary := Summarize([]Result{
		{ID: "a", Level: Must, Status: Pass},
		{ID: "b", Level: Should, Status: Pass},
	})
	if summary.Color != Green {
		t.Fatalf("expected green, got %s", summary.Color)
	}
}

func TestSummarizeOrange(t *testing.T) {
	summary := Summarize([]Result{
		{ID: "a", Level: Must, Status: Pass},
		{ID: "b", Level: Should, Status: Unsupported},
	})
	if summary.Color != Orange {
		t.Fatalf("expected orange, got %s", summary.Color)
	}
}

func TestSummarizeRed(t *testing.T) {
	summary := Summarize([]Result{
		{ID: "a", Level: Should, Status: Unsupported},
		{ID: "b", Level: Must, Status: Fail},
	})
	if summary.Color != Red {
		t.Fatalf("expected red, got %s", summary.Color)
	}
}

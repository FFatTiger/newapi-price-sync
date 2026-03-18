package normalize

import "testing"

func TestModelRatioFromUSDPer1M(t *testing.T) {
	got := ModelRatioFromUSDPer1M(2.5, 1, 1)
	if got != 1.25 {
		t.Fatalf("expected 1.25, got %v", got)
	}
}

func TestModelRatioFromUSDPer1MWithExchange(t *testing.T) {
	got := ModelRatioFromUSDPer1M(2.5, 7.2, 1)
	if got != 9.0 {
		t.Fatalf("expected 9.0, got %v", got)
	}
}

func TestRatio(t *testing.T) {
	got, ok := Ratio(10, 2.5)
	if !ok || got != 4 {
		t.Fatalf("expected 4,true got %v,%v", got, ok)
	}
}

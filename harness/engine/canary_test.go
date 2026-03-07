package engine

import "testing"

func TestShouldUseCanaryEdges(t *testing.T) {
	if ShouldUseCanary("run-1", CanaryPolicy{Enabled: false, Percent: 50}) {
		t.Fatal("expected disabled canary to be false")
	}
	if ShouldUseCanary("run-1", CanaryPolicy{Enabled: true, Percent: 0}) {
		t.Fatal("expected 0% canary to be false")
	}
	if !ShouldUseCanary("run-1", CanaryPolicy{Enabled: true, Percent: 100}) {
		t.Fatal("expected 100% canary to be true")
	}
}

func TestShouldUseCanaryDeterministic(t *testing.T) {
	policy := CanaryPolicy{Enabled: true, Percent: 30, Salt: "v1"}
	first := ShouldUseCanary("run-abc", policy)
	second := ShouldUseCanary("run-abc", policy)
	if first != second {
		t.Fatalf("expected deterministic canary result, got %v then %v", first, second)
	}
}

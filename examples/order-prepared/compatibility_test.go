package main

import "testing"

func TestCompatibilityOnlyRunsSemanticCallbackWithoutPerformanceCollection(t *testing.T) {
	before := compatibilityOnly
	compatibilityOnly = true
	t.Cleanup(func() { compatibilityOnly = before })
	calls := 0
	got := measure(func() { calls++ })
	if calls != 1 || got != (cost{}) {
		t.Fatal("compatibility callback or unmeasured cost differs", calls, got)
	}
}

package search

import "testing"

func TestAdjustParamsByIntent(t *testing.T) {
	// Navigational intent with default/unset limit should return 1
	lim, scrape := AdjustParamsByIntent(IntentNavigational, 0, true)
	if lim != 1 {
		t.Errorf("expected limit 1, got %d", lim)
	}

	lim, scrape = AdjustParamsByIntent(IntentNavigational, 1, false)
	if lim != 1 || scrape != false {
		t.Errorf("expected (1, false), got (%d, %v)", lim, scrape)
	}

	// Navigational intent with explicit limit > 1 must NOT be clamped to 1
	lim, scrape = AdjustParamsByIntent(IntentNavigational, 5, true)
	if lim != 5 || !scrape {
		t.Errorf("expected (5, true), got (%d, %v)", lim, scrape)
	}

	// Informational intent preserves limit and scrape
	lim, scrape = AdjustParamsByIntent(IntentInformational, 10, true)
	if lim != 10 || !scrape {
		t.Errorf("expected (10, true), got (%d, %v)", lim, scrape)
	}
}

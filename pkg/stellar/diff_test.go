package stellar

import (
	"testing"
)

func TestCalculateStellarSplitsDiff(t *testing.T) {
	localSplits := []StellarSplitConfig{
		{DependencyName: "pkgA", StellarAddress: "GAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAWHF", Percentage: 50},
		{DependencyName: "pkgB", StellarAddress: "GAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAWHE", Percentage: 30},
		{DependencyName: "pkgC", StellarAddress: "GAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAWHD", Percentage: 20},
	}

	previousSplits := []StellarSplitConfig{
		{DependencyName: "pkgA", StellarAddress: "GAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAWHF", Percentage: 40}, // MODIFIED
		{DependencyName: "pkgB", StellarAddress: "GAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAWHE", Percentage: 30}, // UNCHANGED
		{DependencyName: "pkgD", StellarAddress: "GAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAWHC", Percentage: 30}, // REMOVED
	}

	diff := CalculateStellarSplitsDiff("GAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAWHF", "testnet", localSplits, previousSplits)

	if !diff.HasChanges() {
		t.Fatal("expected diff to have changes")
	}

	if diff.AddedCount != 1 {
		t.Errorf("expected 1 added, got %d", diff.AddedCount)
	}
	if diff.ModifiedCount != 1 {
		t.Errorf("expected 1 modified, got %d", diff.ModifiedCount)
	}
	if diff.RemovedCount != 1 {
		t.Errorf("expected 1 removed, got %d", diff.RemovedCount)
	}
	if diff.UnchangedCount != 1 {
		t.Errorf("expected 1 unchanged, got %d", diff.UnchangedCount)
	}
}

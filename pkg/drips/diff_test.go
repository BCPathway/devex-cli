package drips_test

import (
	"testing"

	"github.com/BCPathway/devex-cli/pkg/drips"
)

func TestCalculateSplitsDiff(t *testing.T) {
	localSplits := []drips.SplitConfig{
		{
			DependencyName: "github.com/spf13/cobra",
			DripsAccountID: "drips:1:cobra",
			Percentage:     20, // Modified from 15% -> 20%
		},
		{
			DependencyName: "github.com/new/added-dep",
			DripsAccountID: "drips:1:added",
			Percentage:     5, // Added (new in local)
		},
		{
			DependencyName: "github.com/spf13/viper",
			DripsAccountID: "drips:1:viper",
			Percentage:     5, // Unchanged (5% -> 5%)
		},
	}

	onChainSplits := []drips.OnChainSplitEntry{
		{
			ReceiverAccountID: "drips:1:cobra",
			Percentage:        15,
		},
		{
			ReceiverAccountID: "drips:1:viper",
			Percentage:        5,
		},
		{
			ReceiverAccountID: "drips:1:old-removed",
			Percentage:        10, // Removed (on-chain but not in local)
		},
	}

	diff := drips.CalculateSplitsDiff("0xTestProject", 10, localSplits, onChainSplits)

	if !diff.HasChanges() {
		t.Fatal("expected HasChanges() == true")
	}

	if diff.AddedCount != 1 {
		t.Errorf("AddedCount = %d, want 1", diff.AddedCount)
	}
	if diff.ModifiedCount != 1 {
		t.Errorf("ModifiedCount = %d, want 1", diff.ModifiedCount)
	}
	if diff.RemovedCount != 1 {
		t.Errorf("RemovedCount = %d, want 1", diff.RemovedCount)
	}
	if diff.UnchangedCount != 1 {
		t.Errorf("UnchangedCount = %d, want 1", diff.UnchangedCount)
	}

	// Verify order: Added -> Modified -> Removed -> Unchanged
	if len(diff.Items) != 4 {
		t.Fatalf("len(Items) = %d, want 4", len(diff.Items))
	}
	if diff.Items[0].Type != drips.DiffAdded {
		t.Errorf("first item type = %s, want ADDED", diff.Items[0].Type)
	}
	if diff.Items[1].Type != drips.DiffModified {
		t.Errorf("second item type = %s, want MODIFIED", diff.Items[1].Type)
	}
	if diff.Items[2].Type != drips.DiffRemoved {
		t.Errorf("third item type = %s, want REMOVED", diff.Items[2].Type)
	}
	if diff.Items[3].Type != drips.DiffUnchanged {
		t.Errorf("fourth item type = %s, want UNCHANGED", diff.Items[3].Type)
	}
}

func TestSplitsDiff_NoChanges(t *testing.T) {
	localSplits := []drips.SplitConfig{
		{
			DependencyName: "github.com/spf13/cobra",
			DripsAccountID: "drips:1:cobra",
			Percentage:     15,
		},
	}

	onChainSplits := []drips.OnChainSplitEntry{
		{
			ReceiverAccountID: "drips:1:cobra",
			Percentage:        15,
		},
	}

	diff := drips.CalculateSplitsDiff("0xTestProject", 10, localSplits, onChainSplits)

	if diff.HasChanges() {
		t.Errorf("expected HasChanges() == false, got true")
	}
}

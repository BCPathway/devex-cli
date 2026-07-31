package drips_test

import (
	"path/filepath"
	"testing"

	"github.com/BCPathway/devex-cli/pkg/drips"
)

func TestSaveAndLoadDripsConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".devex.drips.yaml")

	original := &drips.DripsConfigFile{
		ProjectID: "drips:1:myproject",
		Splits: []drips.SplitConfig{
			{
				DependencyName: "github.com/spf13/cobra",
				DripsAccountID: "drips:1:cobra",
				Address:        "0x1111111111111111111111111111111111111111",
				Percentage:     15,
				Locked:         true,
			},
			{
				DependencyName: "github.com/spf13/viper",
				DripsAccountID: "drips:1:viper",
				Percentage:     5,
				Locked:         false,
			},
		},
	}

	if err := drips.SaveDripsConfig(path, original); err != nil {
		t.Fatalf("SaveDripsConfig returned error: %v", err)
	}

	loaded, err := drips.LoadDripsConfig(path)
	if err != nil {
		t.Fatalf("LoadDripsConfig returned error: %v", err)
	}

	if loaded.ProjectID != original.ProjectID {
		t.Errorf("ProjectID = %q, want %q", loaded.ProjectID, original.ProjectID)
	}

	if len(loaded.Splits) != len(original.Splits) {
		t.Fatalf("len(Splits) = %d, want %d", len(loaded.Splits), len(original.Splits))
	}

	if loaded.Splits[0].DependencyName != "github.com/spf13/cobra" || !loaded.Splits[0].Locked {
		t.Errorf("unexpected first split: %+v", loaded.Splits[0])
	}
}

func TestMergeDripsConfig_PreservesLocked(t *testing.T) {
	existing := &drips.DripsConfigFile{
		ProjectID: "drips:existing",
		Splits: []drips.SplitConfig{
			{
				DependencyName: "github.com/spf13/cobra",
				Percentage:     50,
				Locked:         true,
			},
			{
				DependencyName: "github.com/old/unlocked",
				Percentage:     10,
				Locked:         false,
			},
		},
	}

	newlyGenerated := &drips.DripsConfigFile{
		ProjectID: "drips:new",
		Splits: []drips.SplitConfig{
			{
				DependencyName: "github.com/spf13/cobra",
				Percentage:     10, // Should be ignored because locked in existing
			},
			{
				DependencyName: "github.com/new/depA",
				Percentage:     20,
			},
			{
				DependencyName: "github.com/new/depB",
				Percentage:     20,
			},
		},
	}

	merged := drips.MergeDripsConfig(existing, newlyGenerated)

	if merged.ProjectID != "drips:new" {
		t.Errorf("ProjectID = %q, want %q", merged.ProjectID, "drips:new")
	}

	// Total should be capped at 100%, cobra should remain 50% locked
	var cobraSplit *drips.SplitConfig
	totalPct := 0
	for i, sp := range merged.Splits {
		totalPct += sp.Percentage
		if sp.DependencyName == "github.com/spf13/cobra" {
			cobraSplit = &merged.Splits[i]
		}
	}

	if cobraSplit == nil {
		t.Fatal("cobra split missing in merged config")
	}
	if cobraSplit.Percentage != 50 || !cobraSplit.Locked {
		t.Errorf("cobra split = %+v, want percentage 50 and locked=true", cobraSplit)
	}

	if totalPct != 100 {
		t.Errorf("total percentage = %d, want 100", totalPct)
	}
}

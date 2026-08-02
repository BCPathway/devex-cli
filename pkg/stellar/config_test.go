package stellar

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveAndLoadStellarConfig(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, ".devex.stellar.yaml")

	cfg := &StellarConfigFile{
		AccountID: "GAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAWHF",
		Network:   "testnet",
		Splits: []StellarSplitConfig{
			{
				DependencyName: "github.com/stellar/go-stellar-sdk",
				StellarAddress: "GAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAWHF",
				Percentage:     50,
				Locked:         true,
			},
			{
				DependencyName: "github.com/spf13/cobra",
				StellarAddress: "GAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAWHE",
				Percentage:     50,
				Locked:         false,
			},
		},
	}

	if err := SaveStellarConfig(path, cfg); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("config file was not created: %v", err)
	}

	loaded, err := LoadStellarConfig(path)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if loaded.AccountID != cfg.AccountID {
		t.Errorf("expected %s, got %s", cfg.AccountID, loaded.AccountID)
	}
	if len(loaded.Splits) != 2 {
		t.Fatalf("expected 2 splits, got %d", len(loaded.Splits))
	}
	if !loaded.Splits[0].Locked {
		t.Errorf("expected first split to be locked")
	}
}

func TestMergeStellarConfig(t *testing.T) {
	existing := &StellarConfigFile{
		AccountID: "GAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAWHF",
		Network:   "testnet",
		Splits: []StellarSplitConfig{
			{
				DependencyName: "github.com/stellar/go-stellar-sdk",
				StellarAddress: "GAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAWHF",
				Percentage:     60,
				Locked:         true,
			},
		},
	}

	generated := &StellarConfigFile{
		AccountID: "GAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAWHF",
		Network:   "testnet",
		Splits: []StellarSplitConfig{
			{
				DependencyName: "github.com/stellar/go-stellar-sdk",
				StellarAddress: "GAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAWHE",
				Percentage:     40,
				Locked:         false,
			},
			{
				DependencyName: "github.com/spf13/cobra",
				StellarAddress: "GAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAWHD",
				Percentage:     40,
				Locked:         false,
			},
		},
	}

	merged := MergeStellarConfig(existing, generated)
	if len(merged.Splits) != 2 {
		t.Fatalf("expected 2 merged splits, got %d", len(merged.Splits))
	}

	// Should preserve the locked entry from existing (60% to WHF).
	if merged.Splits[0].Percentage != 60 || merged.Splits[0].StellarAddress != "GAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAWHF" {
		t.Errorf("locked split was not preserved: %+v", merged.Splits[0])
	}
}

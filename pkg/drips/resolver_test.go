package drips_test

import (
	"testing"

	"github.com/BCPathway/devex-cli/internal/logger"
	"github.com/BCPathway/devex-cli/pkg/drips"
	"github.com/BCPathway/devex-cli/pkg/parser"
)

func init() {
	// Suppress log output during tests.
	logger.Init(false)
}

func TestRegistryResolver_ResolveDependencies_WellKnown(t *testing.T) {
	client, err := drips.NewClient(drips.ClientConfig{
		RPCEndpoint: "https://mainnet.optimism.io",
		ChainID:     10,
	})
	if err != nil {
		t.Fatalf("NewClient() returned error: %v", err)
	}
	defer client.Close()

	resolver := drips.NewRegistryResolver(client)

	deps := []parser.Dependency{
		{Name: "github.com/spf13/cobra", Version: "v1.8.1", Source: parser.ManifestGoMod, Direct: true},
		{Name: "github.com/spf13/viper", Version: "v1.19.0", Source: parser.ManifestGoMod, Direct: true},
		{Name: "github.com/unknown/pkg", Version: "v0.1.0", Source: parser.ManifestGoMod, Direct: true},
	}

	recipients, err := resolver.ResolveDependencies(deps)
	if err != nil {
		t.Fatalf("ResolveDependencies() returned error: %v", err)
	}

	if got := len(recipients); got != 3 {
		t.Fatalf("len(recipients) = %d, want 3", got)
	}

	// cobra should be Verified (from well-known registry).
	cobraRec := findRecipient(recipients, "github.com/spf13/cobra")
	if cobraRec == nil {
		t.Fatal("cobra recipient not found")
	}
	if cobraRec.Status != drips.StatusVerified {
		t.Errorf("cobra status = %q, want %q", cobraRec.Status, drips.StatusVerified)
	}
	if cobraRec.RecommendedSplitPct <= 0 {
		t.Errorf("cobra RecommendedSplitPct = %d, want > 0", cobraRec.RecommendedSplitPct)
	}
	if cobraRec.Source != "well-known" {
		t.Errorf("cobra source = %q, want %q", cobraRec.Source, "well-known")
	}

	// viper should be Escrow (from well-known registry).
	viperRec := findRecipient(recipients, "github.com/spf13/viper")
	if viperRec == nil {
		t.Fatal("viper recipient not found")
	}
	if viperRec.Status != drips.StatusEscrow {
		t.Errorf("viper status = %q, want %q", viperRec.Status, drips.StatusEscrow)
	}

	// unknown/pkg should be Unregistered.
	unknownRec := findRecipient(recipients, "github.com/unknown/pkg")
	if unknownRec == nil {
		t.Fatal("unknown/pkg recipient not found")
	}
	if unknownRec.Status != drips.StatusUnregistered {
		t.Errorf("unknown/pkg status = %q, want %q", unknownRec.Status, drips.StatusUnregistered)
	}
	if unknownRec.RecommendedSplitPct != 0 {
		t.Errorf("unknown/pkg RecommendedSplitPct = %d, want 0", unknownRec.RecommendedSplitPct)
	}
}

func TestRegistryResolver_ResolveDependencies_EmbeddedMetadata(t *testing.T) {
	client, err := drips.NewClient(drips.ClientConfig{
		RPCEndpoint: "https://mainnet.optimism.io",
		ChainID:     10,
	})
	if err != nil {
		t.Fatalf("NewClient() returned error: %v", err)
	}
	defer client.Close()

	resolver := drips.NewRegistryResolver(client)

	deps := []parser.Dependency{
		{
			Name:    "my-custom-lib",
			Version: "^1.0.0",
			Source:  parser.ManifestPackageJSON,
			Direct:  true,
			DripsMetadata: &parser.EmbeddedDripsInfo{
				AccountID: "drips:custom:123",
				Address:   "0xcustomaddress",
			},
		},
	}

	recipients, err := resolver.ResolveDependencies(deps)
	if err != nil {
		t.Fatalf("ResolveDependencies() returned error: %v", err)
	}

	if got := len(recipients); got != 1 {
		t.Fatalf("len(recipients) = %d, want 1", got)
	}

	rec := recipients[0]
	if rec.Status != drips.StatusVerified {
		t.Errorf("status = %q, want %q (embedded metadata should be Verified)", rec.Status, drips.StatusVerified)
	}
	if rec.Source != "manifest" {
		t.Errorf("source = %q, want %q", rec.Source, "manifest")
	}
	if rec.DripsAccountID != "drips:custom:123" {
		t.Errorf("DripsAccountID = %q, want %q", rec.DripsAccountID, "drips:custom:123")
	}
	if rec.Address != "0xcustomaddress" {
		t.Errorf("Address = %q, want %q", rec.Address, "0xcustomaddress")
	}
}

func TestRegistryResolver_ResolveDependencies_Empty(t *testing.T) {
	client, err := drips.NewClient(drips.ClientConfig{
		RPCEndpoint: "https://mainnet.optimism.io",
		ChainID:     10,
	})
	if err != nil {
		t.Fatalf("NewClient() returned error: %v", err)
	}
	defer client.Close()

	resolver := drips.NewRegistryResolver(client)

	recipients, err := resolver.ResolveDependencies([]parser.Dependency{})
	if err != nil {
		t.Fatalf("ResolveDependencies() returned error: %v", err)
	}

	if got := len(recipients); got != 0 {
		t.Errorf("len(recipients) = %d, want 0", got)
	}
}

func TestRegistryResolver_ResolveDependencies_SplitPctHeuristic(t *testing.T) {
	client, err := drips.NewClient(drips.ClientConfig{
		RPCEndpoint: "https://mainnet.optimism.io",
		ChainID:     10,
	})
	if err != nil {
		t.Fatalf("NewClient() returned error: %v", err)
	}
	defer client.Close()

	resolver := drips.NewRegistryResolver(client)

	deps := []parser.Dependency{
		// Direct dep with well-known Verified status → should get 5%.
		{Name: "github.com/spf13/cobra", Version: "v1.8.1", Source: parser.ManifestGoMod, Direct: true},
		// Direct dep with well-known Escrow status → should get 3%.
		{Name: "github.com/spf13/viper", Version: "v1.19.0", Source: parser.ManifestGoMod, Direct: true},
	}

	recipients, err := resolver.ResolveDependencies(deps)
	if err != nil {
		t.Fatalf("ResolveDependencies() returned error: %v", err)
	}

	cobraRec := findRecipient(recipients, "github.com/spf13/cobra")
	if cobraRec.RecommendedSplitPct != 5 {
		t.Errorf("cobra (direct, verified) split = %d, want 5", cobraRec.RecommendedSplitPct)
	}

	viperRec := findRecipient(recipients, "github.com/spf13/viper")
	if viperRec.RecommendedSplitPct != 3 {
		t.Errorf("viper (direct, escrow) split = %d, want 3", viperRec.RecommendedSplitPct)
	}
}

// --------------------------------------------------------------------------
// Test helpers
// --------------------------------------------------------------------------

func findRecipient(recipients []drips.DripsRecipient, name string) *drips.DripsRecipient {
	for i, r := range recipients {
		if r.DependencyName == name {
			return &recipients[i]
		}
	}
	return nil
}

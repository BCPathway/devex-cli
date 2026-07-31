package drips_test

import (
	"context"
	"testing"

	"github.com/BCPathway/devex-cli/pkg/drips"
)

func TestMockStatusTelemetry(t *testing.T) {
	telemetry := drips.MockStatusTelemetry("0x1234567890abcdef1234567890abcdef12345678", 10)

	if telemetry.AccountID != "0x1234567890abcdef1234567890abcdef12345678" {
		t.Errorf("AccountID = %q, want expected address", telemetry.AccountID)
	}

	if telemetry.ChainID != 10 {
		t.Errorf("ChainID = %d, want 10", telemetry.ChainID)
	}

	if len(telemetry.IncomingStreams) != 2 {
		t.Fatalf("len(IncomingStreams) = %d, want 2", len(telemetry.IncomingStreams))
	}

	if len(telemetry.ActiveSplits) != 2 {
		t.Fatalf("len(ActiveSplits) = %d, want 2", len(telemetry.ActiveSplits))
	}

	if telemetry.Source != "offline-simulated" {
		t.Errorf("Source = %q, want offline-simulated", telemetry.Source)
	}
}

func TestSubgraphClient_OfflineFallback(t *testing.T) {
	client := drips.NewSubgraphClient(10)
	telemetry, err := client.QueryStatusTelemetry(context.Background(), "0x1234567890abcdef1234567890abcdef12345678")
	if err != nil {
		t.Fatalf("QueryStatusTelemetry returned error: %v", err)
	}

	if telemetry == nil {
		t.Fatal("expected non-nil telemetry")
	}

	if telemetry.ChainID != 10 {
		t.Errorf("ChainID = %d, want 10", telemetry.ChainID)
	}
}

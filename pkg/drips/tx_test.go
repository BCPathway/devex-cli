package drips_test

import (
	"context"
	"strings"
	"testing"

	"github.com/BCPathway/devex-cli/pkg/drips"
)

func TestBuildSetSplitsPayload_SortingAndWeight(t *testing.T) {
	splits := []drips.SplitConfig{
		{
			DependencyName: "dep-b",
			Address:        "0xBBbbBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB",
			Percentage:     25,
		},
		{
			DependencyName: "dep-a",
			Address:        "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			Percentage:     75,
		},
		{
			DependencyName: "zero-dep",
			Address:        "0xcccccccccccccccccccccccccccccccccccccccc",
			Percentage:     0, // Must be omitted
		},
	}

	payloads, err := drips.BuildSetSplitsPayload(splits)
	if err != nil {
		t.Fatalf("BuildSetSplitsPayload failed: %v", err)
	}

	if len(payloads) != 2 {
		t.Fatalf("len(payloads) = %d, want 2", len(payloads))
	}

	// First should be 0xaaa... (lexicographically before 0xBBb...)
	if !strings.EqualFold(payloads[0].Receiver, "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa") {
		t.Errorf("payloads[0].Receiver = %s, want 0xaaa...", payloads[0].Receiver)
	}
	if payloads[0].Weight != 750000 {
		t.Errorf("payloads[0].Weight = %d, want 750000", payloads[0].Weight)
	}

	if !strings.EqualFold(payloads[1].Receiver, "0xBBbbBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB") {
		t.Errorf("payloads[1].Receiver = %s, want 0xBBB...", payloads[1].Receiver)
	}
	if payloads[1].Weight != 250000 {
		t.Errorf("payloads[1].Weight = %d, want 250000", payloads[1].Weight)
	}
}

func TestCreateSyncTxPlan_OfflineSimulation(t *testing.T) {
	splits := []drips.SplitConfig{
		{
			DependencyName: "dep-a",
			Address:        "0x1234567890123456789012345678901234567890",
			Percentage:     100,
		},
	}

	plan, err := drips.CreateSyncTxPlan(context.Background(), "http://invalid-rpc-url:0", 10, "0xSender", splits)
	if err != nil {
		t.Fatalf("CreateSyncTxPlan failed: %v", err)
	}

	if !plan.IsOfflineSimulation {
		t.Error("expected IsOfflineSimulation = true for invalid RPC")
	}

	if plan.EstimatedGasLimit == 0 {
		t.Error("expected positive EstimatedGasLimit")
	}

	if plan.ContractAddress != drips.DripsHubOptimism {
		t.Errorf("ContractAddress = %s, want %s", plan.ContractAddress, drips.DripsHubOptimism)
	}

	txHash, err := drips.ExecuteSyncTx(context.Background(), "http://invalid-rpc-url:0", 10, "4c0883a69102937d6231471b5dbb6204fe5129617082792ae468d01a3f362318", plan)
	if err != nil {
		t.Fatalf("ExecuteSyncTx failed: %v", err)
	}

	if !strings.HasPrefix(txHash, "0x") || len(txHash) != 66 {
		t.Errorf("expected 66-character hex tx hash, got %s", txHash)
	}
}

package stellar

import (
	"testing"
)

func TestBuildSplitPayments_Success(t *testing.T) {
	splits := []StellarSplitEntry{
		{Receiver: "GAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAWHF", Weight: 50},
		{Receiver: "GAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAWHE", Weight: 30},
		{Receiver: "GAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAWHD", Weight: 20},
	}

	payments, err := BuildSplitPayments("100.0000000", splits)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(payments) != 3 {
		t.Fatalf("expected 3 payments, got %d", len(payments))
	}

	if payments[0].Amount != "50.0000000" {
		t.Errorf("expected 50.0000000, got %s", payments[0].Amount)
	}
	if payments[1].Amount != "30.0000000" {
		t.Errorf("expected 30.0000000, got %s", payments[1].Amount)
	}
	if payments[2].Amount != "20.0000000" {
		t.Errorf("expected 20.0000000, got %s", payments[2].Amount)
	}
}

func TestBuildSplitPayments_Exceeds100Percent(t *testing.T) {
	splits := []StellarSplitEntry{
		{Receiver: "GAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAWHF", Weight: 60},
		{Receiver: "GAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAWHE", Weight: 50},
	}

	_, err := BuildSplitPayments("100.0000000", splits)
	if err == nil {
		t.Fatal("expected error when total weight > 100%, got nil")
	}
}

func TestCreateSplitTxPlan(t *testing.T) {
	splits := []StellarSplitEntry{
		{Receiver: "GAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAWHF", Weight: 100},
	}

	plan, err := CreateSplitTxPlan("GAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAWHF", "10.0000000", splits, "testnet")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if plan.SourceAccount != "GAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAWHF" {
		t.Errorf("expected source account G..., got %s", plan.SourceAccount)
	}
	if plan.NetworkName != "testnet" {
		t.Errorf("expected testnet, got %s", plan.NetworkName)
	}
	if plan.OperationCount != 1 {
		t.Errorf("expected 1 operation, got %d", plan.OperationCount)
	}
	if plan.BaseFeeStroops != 100 {
		t.Errorf("expected 100 stroops base fee, got %d", plan.BaseFeeStroops)
	}
}

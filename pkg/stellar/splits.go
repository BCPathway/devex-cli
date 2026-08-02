package stellar

import (
	"context"
	"fmt"
	"strconv"

	"github.com/BCPathway/devex-cli/internal/logger"
	"github.com/stellar/go-stellar-sdk/clients/horizonclient"
	"github.com/stellar/go-stellar-sdk/keypair"
	"github.com/stellar/go-stellar-sdk/network"
	"github.com/stellar/go-stellar-sdk/txnbuild"
)

// SplitPayment represents a single payment within a split distribution.
type SplitPayment struct {
	Destination string  `json:"destination"` // Stellar address (G...)
	Percentage  int     `json:"percentage"`  // 0-100
	Amount      string  `json:"amount"`      // calculated XLM amount
}

// SplitTxPlan contains the payment breakdown and fee details for a split transaction.
type SplitTxPlan struct {
	SourceAccount    string         `json:"source_account"`
	NetworkName      string         `json:"network_name"` // "testnet" or "mainnet"
	TotalAmount      string         `json:"total_amount"` // XLM to distribute
	Payments         []SplitPayment `json:"payments"`
	BaseFeeStroops   int64          `json:"base_fee_stroops"` // 100 stroops per op
	TotalFeeXLM      string         `json:"total_fee_xlm"`
	OperationCount   int            `json:"operation_count"`
	IsSimulation     bool           `json:"is_simulation"`
}

// BuildSplitPayments calculates the XLM amount for each recipient based on percentages.
// totalXLM is specified as a decimal string (e.g. "100.0000000").
func BuildSplitPayments(totalXLM string, splits []StellarSplitEntry) ([]SplitPayment, error) {
	total, err := strconv.ParseFloat(totalXLM, 64)
	if err != nil {
		return nil, fmt.Errorf("stellar: invalid total amount %q: %w", totalXLM, err)
	}

	if total <= 0 {
		return nil, fmt.Errorf("stellar: total amount must be positive, got %s", totalXLM)
	}

	// Validate total percentage.
	totalPct := 0
	for _, s := range splits {
		if s.Weight < 0 || s.Weight > 100 {
			return nil, fmt.Errorf("stellar: invalid split weight %d for %s", s.Weight, s.Receiver)
		}
		totalPct += s.Weight
	}
	if totalPct > 100 {
		return nil, fmt.Errorf("stellar: total split weight %d%% exceeds 100%%", totalPct)
	}

	payments := make([]SplitPayment, 0, len(splits))
	for _, s := range splits {
		if s.Weight <= 0 {
			continue
		}
		amount := total * float64(s.Weight) / 100.0
		payments = append(payments, SplitPayment{
			Destination: s.Receiver,
			Percentage:  s.Weight,
			Amount:      fmt.Sprintf("%.7f", amount),
		})
	}

	return payments, nil
}

// CreateSplitTxPlan builds a transaction plan for distributing XLM to split receivers.
func CreateSplitTxPlan(sourceAccount string, totalXLM string, splits []StellarSplitEntry, networkName string) (*SplitTxPlan, error) {
	payments, err := BuildSplitPayments(totalXLM, splits)
	if err != nil {
		return nil, err
	}

	if len(payments) == 0 {
		return nil, fmt.Errorf("stellar: no valid split payments to create")
	}

	// Stellar base fee is 100 stroops (0.00001 XLM) per operation.
	baseFee := int64(100)
	totalFeeStroops := baseFee * int64(len(payments))
	totalFeeXLM := float64(totalFeeStroops) / 1e7

	return &SplitTxPlan{
		SourceAccount:  sourceAccount,
		NetworkName:    networkName,
		TotalAmount:    totalXLM,
		Payments:       payments,
		BaseFeeStroops: baseFee,
		TotalFeeXLM:    fmt.Sprintf("%.7f", totalFeeXLM),
		OperationCount: len(payments),
		IsSimulation:   false,
	}, nil
}

// ExecuteSplitTx builds, signs, and submits a Stellar transaction with
// multiple Payment operations to distribute funds according to the plan.
func ExecuteSplitTx(_ context.Context, horizonURL string, networkPassphrase string, secretKey string, plan *SplitTxPlan) (string, error) {
	if plan.IsSimulation {
		logger.Info("stellar: simulation mode — generating mock transaction hash")
		return "sim_" + plan.SourceAccount[:8] + "_stellar_split", nil
	}

	// Parse the secret key to get the keypair.
	kp, err := keypair.ParseFull(secretKey)
	if err != nil {
		return "", fmt.Errorf("stellar: invalid secret key: %w", err)
	}

	// Create Horizon client.
	client := &horizonclient.Client{
		HorizonURL: horizonURL,
	}

	// Load the source account.
	sourceAccount, err := client.AccountDetail(horizonclient.AccountRequest{
		AccountID: kp.Address(),
	})
	if err != nil {
		return "", fmt.Errorf("stellar: loading source account: %w", err)
	}

	// Build payment operations.
	ops := make([]txnbuild.Operation, 0, len(plan.Payments))
	for _, p := range plan.Payments {
		ops = append(ops, &txnbuild.Payment{
			Destination: p.Destination,
			Amount:      p.Amount,
			Asset:       txnbuild.NativeAsset{},
		})
	}

	// Build and sign the transaction.
	tx, err := txnbuild.NewTransaction(
		txnbuild.TransactionParams{
			SourceAccount:        &sourceAccount,
			IncrementSequenceNum: true,
			Operations:           ops,
			BaseFee:              plan.BaseFeeStroops,
			Preconditions: txnbuild.Preconditions{
				TimeBounds: txnbuild.NewInfiniteTimeout(),
			},
			Memo: txnbuild.MemoText("devex funding split"),
		},
	)
	if err != nil {
		return "", fmt.Errorf("stellar: building transaction: %w", err)
	}

	// Resolve the network passphrase.
	passphrase := networkPassphrase
	if passphrase == "" {
		passphrase = network.TestNetworkPassphrase
	}

	tx, err = tx.Sign(passphrase, kp)
	if err != nil {
		return "", fmt.Errorf("stellar: signing transaction: %w", err)
	}

	// Submit the transaction.
	resp, err := client.SubmitTransaction(tx)
	if err != nil {
		logger.Warn("stellar: transaction submission failed (%v)", err)
		return "", fmt.Errorf("stellar: submitting transaction: %w", err)
	}

	logger.Info("stellar: transaction submitted successfully — hash: %s", resp.Hash)
	return resp.Hash, nil
}

// EstimateFees returns the estimated fee for a split transaction.
// Stellar fees are simple and predictable: 100 stroops per operation.
func EstimateFees(operationCount int) (stroops int64, xlm string) {
	stroops = int64(operationCount) * 100
	xlmVal := float64(stroops) / 1e7
	return stroops, fmt.Sprintf("%.7f XLM", xlmVal)
}

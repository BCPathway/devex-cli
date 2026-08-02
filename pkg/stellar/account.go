package stellar

import (
	"fmt"

	"github.com/BCPathway/devex-cli/internal/logger"
	"github.com/stellar/go-stellar-sdk/clients/horizonclient"
	"github.com/stellar/go-stellar-sdk/protocols/horizon"
	"github.com/stellar/go-stellar-sdk/protocols/horizon/operations"
)

// GetBalance retrieves the XLM and other asset balances for the given account.
func (c *Client) GetBalance(accountID string) ([]StellarBalanceInfo, error) {
	if accountID == "" {
		return nil, fmt.Errorf("stellar: account ID is required")
	}

	logger.Debug("stellar: querying balance for %s", accountID)

	account, err := c.horizon.AccountDetail(horizonclient.AccountRequest{
		AccountID: accountID,
	})
	if err != nil {
		return nil, fmt.Errorf("stellar: querying account %s: %w", accountID, err)
	}

	balances := make([]StellarBalanceInfo, 0, len(account.Balances))
	for _, b := range account.Balances {
		asset := "native"
		if b.Asset.Type != "native" {
			asset = fmt.Sprintf("%s:%s", b.Asset.Code, b.Asset.Issuer)
		}
		balances = append(balances, StellarBalanceInfo{
			AccountID: accountID,
			Asset:     asset,
			Balance:   b.Balance,
		})
	}

	return balances, nil
}

// GetPaymentHistory retrieves recent incoming payments for the given account.
func (c *Client) GetPaymentHistory(accountID string, limit int) ([]StellarPaymentInfo, error) {
	if accountID == "" {
		return nil, fmt.Errorf("stellar: account ID is required")
	}
	if limit <= 0 {
		limit = 10
	}

	logger.Debug("stellar: querying payment history for %s (limit %d)", accountID, limit)

	opRequest := horizonclient.OperationRequest{
		ForAccount: accountID,
		Limit:      uint(limit),
		Order:      horizonclient.OrderDesc,
	}

	ops, err := c.horizon.Operations(opRequest)
	if err != nil {
		return nil, fmt.Errorf("stellar: querying operations for %s: %w", accountID, err)
	}

	payments := make([]StellarPaymentInfo, 0)
	for _, record := range ops.Embedded.Records {
		payment, ok := record.(operations.Payment)
		if !ok {
			continue
		}

		asset := "native"
		if payment.Asset.Type != "native" {
			asset = fmt.Sprintf("%s:%s", payment.Asset.Code, payment.Asset.Issuer)
		}

		payments = append(payments, StellarPaymentInfo{
			From:      payment.From,
			To:        payment.To,
			Amount:    payment.Amount,
			Asset:     asset,
			Timestamp: payment.LedgerCloseTime.Format("2006-01-02T15:04:05Z"),
			TxHash:    payment.GetTransactionHash(),
		})
	}

	return payments, nil
}

// GetAccountSequence returns the current sequence number for transaction building.
func (c *Client) GetAccountSequence(accountID string) (int64, error) {
	account, err := c.horizon.AccountDetail(horizonclient.AccountRequest{
		AccountID: accountID,
	})
	if err != nil {
		return 0, fmt.Errorf("stellar: fetching account detail: %w", err)
	}

	seq, err := account.GetSequenceNumber()
	if err != nil {
		return 0, fmt.Errorf("stellar: parsing sequence number: %w", err)
	}

	return seq, nil
}

// AccountExists checks whether a Stellar account is funded and active.
func (c *Client) AccountExists(accountID string) bool {
	_, err := c.horizon.AccountDetail(horizonclient.AccountRequest{
		AccountID: accountID,
	})
	return err == nil
}

// GetAccount retrieves the full account detail from Horizon.
func (c *Client) GetAccount(accountID string) (*horizon.Account, error) {
	account, err := c.horizon.AccountDetail(horizonclient.AccountRequest{
		AccountID: accountID,
	})
	if err != nil {
		return nil, fmt.Errorf("stellar: fetching account %s: %w", accountID, err)
	}
	return &account, nil
}

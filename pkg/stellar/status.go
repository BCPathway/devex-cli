package stellar

import (
	"fmt"
	"strings"

	"github.com/BCPathway/devex-cli/internal/logger"
	"github.com/stellar/go-stellar-sdk/clients/horizonclient"
)

// StellarStatusTelemetry holds the live Stellar funding state for a project/account.
type StellarStatusTelemetry struct {
	AccountID      string               `json:"account_id"`
	Network        string               `json:"network"` // "testnet" or "mainnet"
	XLMBalance     string               `json:"xlm_balance"`
	OtherBalances  []StellarBalanceInfo  `json:"other_balances,omitempty"`
	RecentPayments []StellarPaymentInfo  `json:"recent_payments"`
	SplitConfig    []StellarSplitConfig  `json:"split_config,omitempty"` // from local config if available
	Source         string               `json:"source"` // "horizon" or "offline-simulated"
}

// QueryStatusTelemetry queries the Horizon API for live account state.
// If the Horizon endpoint is unreachable, it returns simulated telemetry
// so local CLI workflows and testing can proceed seamlessly.
func (c *Client) QueryStatusTelemetry(accountID string) (*StellarStatusTelemetry, error) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return nil, fmt.Errorf("stellar: account ID is required")
	}

	networkName := "testnet"
	if strings.Contains(c.config.HorizonURL, "horizon.stellar.org") &&
		!strings.Contains(c.config.HorizonURL, "testnet") {
		networkName = "mainnet"
	}

	logger.Debug("stellar: querying telemetry for %s on %s (%s)", accountID, networkName, c.config.HorizonURL)

	// Query account detail.
	account, err := c.horizon.AccountDetail(horizonclient.AccountRequest{
		AccountID: accountID,
	})
	if err != nil {
		logger.Warn("stellar: Horizon unreachable (%v), using offline-simulated telemetry", err)
		return MockStatusTelemetry(accountID, networkName), nil
	}

	telemetry := &StellarStatusTelemetry{
		AccountID:     accountID,
		Network:       networkName,
		XLMBalance:    "0.0000000",
		OtherBalances: make([]StellarBalanceInfo, 0),
		Source:        "horizon",
	}

	// Extract balances.
	for _, b := range account.Balances {
		if b.Asset.Type == "native" {
			telemetry.XLMBalance = b.Balance + " XLM"
		} else {
			asset := fmt.Sprintf("%s:%s", b.Asset.Code, b.Asset.Issuer)
			telemetry.OtherBalances = append(telemetry.OtherBalances, StellarBalanceInfo{
				AccountID: accountID,
				Asset:     asset,
				Balance:   b.Balance,
			})
		}
	}

	// Query recent payments.
	payments, err := c.GetPaymentHistory(accountID, 5)
	if err != nil {
		logger.Warn("stellar: failed to fetch payment history (%v)", err)
		telemetry.RecentPayments = []StellarPaymentInfo{}
	} else {
		telemetry.RecentPayments = payments
	}

	return telemetry, nil
}

// MockStatusTelemetry generates realistic simulated Stellar telemetry for offline testing.
func MockStatusTelemetry(accountID string, networkName string) *StellarStatusTelemetry {
	if !strings.HasPrefix(accountID, "G") {
		accountID = "GDEMO...MOCK"
	}

	return &StellarStatusTelemetry{
		AccountID:  accountID,
		Network:    networkName,
		XLMBalance: "1,250.0000000 XLM",
		OtherBalances: []StellarBalanceInfo{},
		RecentPayments: []StellarPaymentInfo{
			{
				From:      "GBCONT...RIBUTOR1",
				To:        accountID,
				Amount:    "500.0000000",
				Asset:     "native",
				Timestamp: "2025-07-15T10:30:00Z",
				TxHash:    "abc123...simulated",
			},
			{
				From:      "GDONO...R2",
				To:        accountID,
				Amount:    "250.0000000",
				Asset:     "native",
				Timestamp: "2025-07-10T14:20:00Z",
				TxHash:    "def456...simulated",
			},
		},
		Source: "offline-simulated",
	}
}

// Package stellar provides the integration layer for the Stellar Network.
// It abstracts Horizon API connectivity, account queries, payment
// operations, and split configuration behind a clean Go client.
//
// Architecture:
//   - client.go: Client lifecycle, Horizon connection management
//   - types.go: Shared domain types
//   - account.go: Account balance and payment history queries
//   - splits.go: Split payment builder and executor
//   - config.go: Local config file (.devex.stellar.yaml) schema & merge logic
//   - status.go: Status telemetry (live + mock)
//   - diff.go: State diff engine
//
// This package is intentionally placed under pkg/ (not internal/) so that
// external tools can import and reuse the Stellar client if needed.
package stellar

import (
	"fmt"

	"github.com/BCPathway/devex-cli/internal/logger"
	"github.com/stellar/go-stellar-sdk/clients/horizonclient"
)

// Horizon endpoint constants.
const (
	HorizonMainnet = "https://horizon.stellar.org"
	HorizonTestnet = "https://horizon-testnet.stellar.org"
)

// Network passphrase constants.
const (
	NetworkMainnet = "Public Global Stellar Network ; September 2015"
	NetworkTestnet = "Test SDF Network ; September 2015"
)

// ClientConfig holds the parameters needed to initialise a Stellar client.
type ClientConfig struct {
	HorizonURL        string
	NetworkPassphrase string
	AccountID         string // Stellar public key (G...)
	SecretKey         string // optional — only needed for write operations
}

// Client is the primary interface to the Stellar Network. It wraps a
// Horizon API client and provides high-level methods for querying and
// managing Stellar state.
type Client struct {
	config ClientConfig
	// horizon holds the Horizon API client.
	horizon *horizonclient.Client
}

// NewClient creates a new Stellar client and validates the configuration.
// It eagerly creates the Horizon client since it's lightweight.
func NewClient(cfg ClientConfig) (*Client, error) {
	if cfg.HorizonURL == "" {
		return nil, fmt.Errorf("stellar: Horizon URL is required")
	}
	if cfg.NetworkPassphrase == "" {
		return nil, fmt.Errorf("stellar: network passphrase is required")
	}

	logger.Debug("stellar: client created for %s", cfg.HorizonURL)

	return &Client{
		config: cfg,
		horizon: &horizonclient.Client{
			HorizonURL: cfg.HorizonURL,
		},
	}, nil
}

// Close releases any resources held by the client.
func (c *Client) Close() {
	c.horizon = nil
	logger.Debug("stellar: client closed")
}

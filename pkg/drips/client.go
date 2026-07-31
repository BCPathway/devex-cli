// Package drips provides the integration layer for the Drips Network
// (https://drips.network). It abstracts RPC connectivity, stream queries,
// balance lookups, and split configuration behind a clean Go client.
//
// Architecture:
//   - client.go: Client lifecycle, RPC connection management
//   - streams.go: Stream query operations
//   - splits.go: Split rule management
//   - types.go: Shared domain types
//
// This package is intentionally placed under pkg/ (not internal/) so that
// external tools can import and reuse the Drips client if needed.
package drips

import (
	"fmt"

	"github.com/BCPathway/devex-cli/internal/logger"
)

// ClientConfig holds the parameters needed to initialise a Drips client.
type ClientConfig struct {
	RPCEndpoint   string
	ChainID       int
	WalletAddress string
	PrivateKey    string // optional — only needed for write operations
}

// Client is the primary interface to the Drips Network. It wraps an
// Ethereum JSON-RPC connection and provides high-level methods for
// querying and managing Drips state.
type Client struct {
	config ClientConfig
	// rpcClient would hold the go-ethereum ethclient.Client in a real
	// implementation. Kept as interface{} during scaffolding to avoid
	// requiring a live RPC endpoint at build time.
	rpcClient interface{}
}

// NewClient creates a new Drips client and validates the configuration.
// It does NOT eagerly connect to the RPC endpoint — connections are
// established lazily on first use.
func NewClient(cfg ClientConfig) (*Client, error) {
	if cfg.RPCEndpoint == "" {
		return nil, fmt.Errorf("drips: RPC endpoint is required")
	}
	if cfg.ChainID == 0 {
		return nil, fmt.Errorf("drips: chain ID is required")
	}

	logger.Debug("drips: client created for chain %d via %s", cfg.ChainID, cfg.RPCEndpoint)

	return &Client{
		config: cfg,
	}, nil
}

// connect establishes the RPC connection if not already connected.
// This is called lazily by methods that need network access.
func (c *Client) connect() error {
	if c.rpcClient != nil {
		return nil
	}

	logger.Debug("drips: connecting to %s…", c.config.RPCEndpoint)

	// TODO: Replace with actual ethclient.Dial call:
	//
	//   client, err := ethclient.Dial(c.config.RPCEndpoint)
	//   if err != nil {
	//       return fmt.Errorf("drips: failed to connect to RPC: %w", err)
	//   }
	//   c.rpcClient = client
	//
	// For now, we simulate a successful connection.
	c.rpcClient = struct{}{}

	logger.Debug("drips: connected successfully")
	return nil
}

// Close releases any resources held by the client.
func (c *Client) Close() {
	// TODO: Close ethclient connection when real client is wired up.
	c.rpcClient = nil
	logger.Debug("drips: client closed")
}

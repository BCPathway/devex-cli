package drips

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/BCPathway/devex-cli/internal/logger"
)

// StatusTelemetry holds the live on-chain Drips funding state for a project/account.
type StatusTelemetry struct {
	AccountID          string               `json:"account_id"`
	Address            string               `json:"address"`
	ChainID            int                  `json:"chain_id"`
	SplittableBalance  string               `json:"splittable_balance"`
	CollectableBalance string               `json:"collectable_balance"`
	IncomingStreams    []IncomingStreamInfo `json:"incoming_streams"`
	ActiveSplits       []OnChainSplitEntry  `json:"active_splits"`
	Source             string               `json:"source"` // "subgraph" or "offline-simulated"
}

// IncomingStreamInfo represents an active stream flowing into the account.
type IncomingStreamInfo struct {
	SenderAccountID string `json:"sender_account_id"`
	SenderAddress   string `json:"sender_address"`
	AmtPerSec       string `json:"amt_per_sec"` // wei per second
	TokenAddress    string `json:"token_address"`
	TokenSymbol     string `json:"token_symbol"`
}

// OnChainSplitEntry represents a split receiver configured on-chain.
type OnChainSplitEntry struct {
	ReceiverAccountID string `json:"receiver_account_id"`
	ReceiverAddress   string `json:"receiver_address"`
	Percentage        int    `json:"percentage"`
	Weight            int    `json:"weight"`
}

// SubgraphClient handles GraphQL queries against the Drips Network subgraph.
type SubgraphClient struct {
	endpoint   string
	chainID    int
	httpClient *http.Client
}

// NewSubgraphClient creates a new subgraph client with a 10-second timeout.
func NewSubgraphClient(chainID int) *SubgraphClient {
	return &SubgraphClient{
		endpoint: resolveSubgraphURL(chainID),
		chainID:  chainID,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// QueryStatusTelemetry queries the Drips Subgraph for live balances, streams, and splits.
// If the subgraph is unreachable or network calls fail, it returns simulated telemetry
// so local CLI workflows and testing can proceed seamlessly.
func (s *SubgraphClient) QueryStatusTelemetry(ctx context.Context, accountIDOrAddress string) (*StatusTelemetry, error) {
	accountIDOrAddress = strings.TrimSpace(accountIDOrAddress)
	if accountIDOrAddress == "" {
		return nil, fmt.Errorf("drips subgraph: account ID or address is required")
	}

	logger.Debug("drips subgraph: querying telemetry for %s on chain %d (%s)",
		accountIDOrAddress, s.chainID, s.endpoint)

	query := `
	query GetAccountStatus($id: ID!) {
	  user(id: $id) {
	    id
	    address
	    assetConfigs {
	      tokenAddress
	      splittableBalance
	      collectableBalance
	    }
	    incomingStreams(where: { status: ACTIVE }) {
	      id
	      sender {
	        id
	        address
	      }
	      amtPerSec
	      tokenAddress
	    }
	    splitsEntries {
	      receiver {
	        id
	        address
	      }
	      weight
	    }
	  }
	}`

	reqBody := map[string]any{
		"query": query,
		"variables": map[string]any{
			"id": strings.ToLower(accountIDOrAddress),
		},
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshalling GraphQL query: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, bytes.NewBuffer(payload))
	if err != nil {
		return nil, fmt.Errorf("creating HTTP request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		logger.Warn("subgraph unreachable (%v), using offline-simulated telemetry", err)
		return MockStatusTelemetry(accountIDOrAddress, s.chainID), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.Warn("subgraph returned HTTP %d, using offline-simulated telemetry", resp.StatusCode)
		return MockStatusTelemetry(accountIDOrAddress, s.chainID), nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Warn("failed reading subgraph response (%v), using offline-simulated telemetry", err)
		return MockStatusTelemetry(accountIDOrAddress, s.chainID), nil
	}

	var graphResp subgraphTelemetryResponse
	if err := json.Unmarshal(body, &graphResp); err != nil {
		logger.Warn("failed decoding GraphQL response (%v), using offline-simulated telemetry", err)
		return MockStatusTelemetry(accountIDOrAddress, s.chainID), nil
	}

	if len(graphResp.Errors) > 0 {
		logger.Warn("subgraph query error (%s), using offline-simulated telemetry", graphResp.Errors[0].Message)
		return MockStatusTelemetry(accountIDOrAddress, s.chainID), nil
	}

	if graphResp.Data.User == nil {
		logger.Debug("account %s not found on subgraph, returning empty telemetry", accountIDOrAddress)
		return &StatusTelemetry{
			AccountID:          accountIDOrAddress,
			Address:            accountIDOrAddress,
			ChainID:            s.chainID,
			SplittableBalance:  "0.0000 ETH",
			CollectableBalance: "0.0000 ETH",
			IncomingStreams:    []IncomingStreamInfo{},
			ActiveSplits:       []OnChainSplitEntry{},
			Source:             "subgraph",
		}, nil
	}

	telemetry := parseSubgraphUser(graphResp.Data.User, s.chainID)
	telemetry.Source = "subgraph"
	return telemetry, nil
}

// MockStatusTelemetry generates realistic simulated on-chain telemetry for offline testing.
func MockStatusTelemetry(accountIDOrAddress string, chainID int) *StatusTelemetry {
	address := accountIDOrAddress
	if !strings.HasPrefix(address, "0x") {
		address = "0x71C...89A0"
	}

	return &StatusTelemetry{
		AccountID:          accountIDOrAddress,
		Address:            address,
		ChainID:            chainID,
		SplittableBalance:  "2.4500 ETH (2,450,000,000,000,000,000 wei)",
		CollectableBalance: "0.8250 ETH (825,000,000,000,000,000 wei)",
		IncomingStreams: []IncomingStreamInfo{
			{
				SenderAccountID: "drips:1:optimism-grant",
				SenderAddress:   "0x4200000000000000000000000000000000000042",
				AmtPerSec:       "385802469135802", // ~1 ETH/month
				TokenAddress:    "0x4200000000000000000000000000000000000006",
				TokenSymbol:     "ETH",
			},
			{
				SenderAccountID: "drips:1:gitcoin-donor",
				SenderAddress:   "0x1111111111111111111111111111111111111111",
				AmtPerSec:       "192901234567901", // ~0.5 ETH/month
				TokenAddress:    "0x4200000000000000000000000000000000000006",
				TokenSymbol:     "ETH",
			},
		},
		ActiveSplits: []OnChainSplitEntry{
			{
				ReceiverAccountID: "drips:1:cobra",
				ReceiverAddress:   "0x1234567890abcdef1234567890abcdef12345678",
				Percentage:        15,
				Weight:            150000,
			},
			{
				ReceiverAccountID: "drips:1:viper",
				ReceiverAddress:   "0x234567890abcdef1234567890abcdef123456789",
				Percentage:        5,
				Weight:            50000,
			},
		},
		Source: "offline-simulated",
	}
}

type subgraphTelemetryResponse struct {
	Data   subgraphDataResponse `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

type subgraphDataResponse struct {
	User *subgraphUser `json:"user"`
}

type subgraphUser struct {
	ID           string `json:"id"`
	Address      string `json:"address"`
	AssetConfigs []struct {
		TokenAddress       string `json:"tokenAddress"`
		SplittableBalance  string `json:"splittableBalance"`
		CollectableBalance string `json:"collectableBalance"`
	} `json:"assetConfigs"`
	IncomingStreams []struct {
		ID     string `json:"id"`
		Sender struct {
			ID      string `json:"id"`
			Address string `json:"address"`
		} `json:"sender"`
		AmtPerSec    string `json:"amtPerSec"`
		TokenAddress string `json:"tokenAddress"`
	} `json:"incomingStreams"`
	SplitsEntries []struct {
		Receiver struct {
			ID      string `json:"id"`
			Address string `json:"address"`
		} `json:"receiver"`
		Weight string `json:"weight"`
	} `json:"splitsEntries"`
}

func parseSubgraphUser(u *subgraphUser, chainID int) *StatusTelemetry {
	t := &StatusTelemetry{
		AccountID:          u.ID,
		Address:            u.Address,
		ChainID:            chainID,
		SplittableBalance:  "0.0000 ETH",
		CollectableBalance: "0.0000 ETH",
		IncomingStreams:    make([]IncomingStreamInfo, 0, len(u.IncomingStreams)),
		ActiveSplits:       make([]OnChainSplitEntry, 0, len(u.SplitsEntries)),
	}

	if len(u.AssetConfigs) > 0 {
		t.SplittableBalance = u.AssetConfigs[0].SplittableBalance + " wei"
		t.CollectableBalance = u.AssetConfigs[0].CollectableBalance + " wei"
	}

	for _, s := range u.IncomingStreams {
		t.IncomingStreams = append(t.IncomingStreams, IncomingStreamInfo{
			SenderAccountID: s.Sender.ID,
			SenderAddress:   s.Sender.Address,
			AmtPerSec:       s.AmtPerSec,
			TokenAddress:    s.TokenAddress,
			TokenSymbol:     "ETH",
		})
	}

	totalWeight := 0
	weights := make([]int, len(u.SplitsEntries))
	for i, sp := range u.SplitsEntries {
		w, _ := strconv.Atoi(sp.Weight)
		weights[i] = w
		totalWeight += w
	}

	for i, sp := range u.SplitsEntries {
		pct := 0
		if totalWeight > 0 {
			pct = (weights[i] * 100) / totalWeight
		}
		t.ActiveSplits = append(t.ActiveSplits, OnChainSplitEntry{
			ReceiverAccountID: sp.Receiver.ID,
			ReceiverAddress:   sp.Receiver.Address,
			Percentage:        pct,
			Weight:            weights[i],
		})
	}

	return t
}

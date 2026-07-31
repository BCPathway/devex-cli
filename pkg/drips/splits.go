package drips

import "fmt"

// GetSplits retrieves the current split configuration for the given account.
//
// TODO: Implement actual contract call to DripsHub.getSplits()
func (c *Client) GetSplits(accountID string) ([]SplitEntry, error) {
	if err := c.connect(); err != nil {
		return nil, err
	}

	if accountID == "" {
		return nil, fmt.Errorf("drips: account ID is required")
	}

	// Placeholder: return empty slice.
	// Replace with actual contract interaction:
	//
	//   dripsHub, err := contracts.NewDripsHub(dripsHubAddr, c.rpcClient)
	//   if err != nil { return nil, err }
	//   rawSplits, err := dripsHub.SplitsOf(nil, accountID)
	//   ...
	//
	return []SplitEntry{}, nil
}

// SetSplits submits a new split configuration on-chain for the given account.
// Returns the transaction hash on success.
//
// TODO: Implement actual contract write call via DripsHub.setSplits()
func (c *Client) SetSplits(accountID string, entries []SplitEntry) (string, error) {
	if err := c.connect(); err != nil {
		return "", err
	}

	if accountID == "" {
		return "", fmt.Errorf("drips: account ID is required for setting splits")
	}
	if len(entries) == 0 {
		return "", fmt.Errorf("drips: at least one split entry is required")
	}

	// Validate total weight.
	total := 0
	for _, e := range entries {
		if e.Weight < 0 || e.Weight > 100 {
			return "", fmt.Errorf("drips: invalid split weight %d for %s", e.Weight, e.Receiver)
		}
		total += e.Weight
	}
	if total > 100 {
		return "", fmt.Errorf("drips: total split weight %d%% exceeds 100%%", total)
	}

	// Placeholder: return a mock transaction hash.
	// Replace with actual signed transaction submission:
	//
	//   auth, err := bind.NewKeyedTransactorWithChainID(privateKey, chainID)
	//   if err != nil { return "", err }
	//   tx, err := dripsHub.SetSplits(auth, splits)
	//   if err != nil { return "", err }
	//   return tx.Hash().Hex(), nil
	//
	return "0x" + fmt.Sprintf("%064x", 0), nil
}

// GetBalance retrieves the current drip balance for the given account.
//
// TODO: Implement actual contract call to DripsHub.getBalance()
func (c *Client) GetBalance(accountID string) (string, error) {
	if err := c.connect(); err != nil {
		return "", err
	}

	if accountID == "" {
		return "", fmt.Errorf("drips: account ID is required")
	}

	// Placeholder: return "0" balance.
	// Replace with actual balance query:
	//
	//   balance, err := dripsHub.Balances(nil, accountID)
	//   if err != nil { return "", err }
	//   return balance.String(), nil
	//
	return "0", nil
}

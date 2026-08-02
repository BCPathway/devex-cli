package stellar

// StellarSplitEntry represents a single split rule in a Stellar split configuration.
type StellarSplitEntry struct {
	Receiver string `json:"receiver"` // Stellar public key (G...)
	Weight   int    `json:"weight"`   // percentage (0-100)
}

// StellarBalanceInfo holds asset balance details for a Stellar account.
type StellarBalanceInfo struct {
	AccountID string `json:"account_id"`
	Asset     string `json:"asset"`   // "native" for XLM, or "CODE:ISSUER"
	Balance   string `json:"balance"` // decimal string (e.g. "100.0000000")
}

// StellarPaymentInfo represents a single payment received by an account.
type StellarPaymentInfo struct {
	From      string `json:"from"`       // sender Stellar address
	To        string `json:"to"`         // receiver Stellar address
	Amount    string `json:"amount"`     // decimal string
	Asset     string `json:"asset"`      // "native" or "CODE:ISSUER"
	Timestamp string `json:"timestamp"`  // ISO8601
	TxHash    string `json:"tx_hash"`    // transaction hash
	Memo      string `json:"memo,omitempty"`
}

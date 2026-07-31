package drips

// StreamInfo represents a single active drip stream.
type StreamInfo struct {
	ID        string `json:"id"`
	Sender    string `json:"sender"`
	Receiver  string `json:"receiver"`
	AmtPerSec string `json:"amt_per_sec"` // wei per second, string to avoid precision loss
	StartTime int64  `json:"start_time"`  // unix timestamp
	Duration  int64  `json:"duration"`    // seconds, 0 = indefinite
}

// SplitEntry represents a single split rule in a Drips split configuration.
type SplitEntry struct {
	Receiver string `json:"receiver"` // Ethereum address
	Weight   int    `json:"weight"`   // percentage (0-100)
}

// BalanceInfo holds token balance details for a Drips account.
type BalanceInfo struct {
	AccountID   string `json:"account_id"`
	TokenAddr   string `json:"token_address"`
	Receivable  string `json:"receivable"`   // claimable balance
	Splittable  string `json:"splittable"`   // balance pending split
	Collectable string `json:"collectable"`  // balance ready to collect
}

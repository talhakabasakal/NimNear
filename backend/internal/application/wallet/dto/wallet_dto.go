package dto

import "github.com/google/uuid"

// WalletBalance is the authenticated wallet balance read model.
// Luna amounts are decimal strings so JavaScript clients cannot lose integer precision.
type WalletBalance struct {
	Address      string `json:"address"`
	BalanceLunas string `json:"balance_lunas"`
	BalanceNIM   string `json:"balance_nim"`
	AccountType  string `json:"account_type,omitempty"`
	Network      string `json:"network,omitempty"`
}

// WalletTransaction is a normalized historic Nimiq transaction.
// Fields that the RPC node omitted are left empty rather than invented.
type WalletTransaction struct {
	Hash        string  `json:"hash"`
	Sender      string  `json:"sender"`
	Recipient   string  `json:"recipient"`
	ValueLunas  string  `json:"value_lunas"`
	ValueNIM    string  `json:"value_nim"`
	BlockNumber *uint64 `json:"block_number,omitempty"`
	Timestamp   *uint64 `json:"timestamp,omitempty"`
	NetworkID   *uint64 `json:"network_id,omitempty"`
	Network     string  `json:"network,omitempty"`
	Status      string  `json:"status,omitempty"`
}

// WalletBalanceResponse is the response envelope for GET /wallet/balance.
type WalletBalanceResponse struct {
	Data WalletBalance `json:"data"`
}

// WalletTransactionsResponse is the response envelope for GET /wallet/transactions.
type WalletTransactionsResponse struct {
	Data        []WalletTransaction `json:"data"`
	NextStartAt string              `json:"next_start_at,omitempty"`
}

// WalletQuery selects one of the authenticated user's verified identities.
type WalletQuery struct {
	UserID  uuid.UUID
	Address string
	Max     int
	StartAt string
}

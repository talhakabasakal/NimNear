package dto

import (
	"time"

	"github.com/google/uuid"
)

// CreateRequest is the authenticated creator input. Recipient and creator are never accepted from the client.
type CreateRequest struct {
	AmountNIM string `json:"amount_nim"`
	Note      string `json:"note"`
	Address   string `json:"address"`
}

// SubmitTransactionRequest accepts only the wallet-produced transaction hash.
type SubmitTransactionRequest struct {
	TransactionHash string `json:"transaction_hash"`
}

// RequestInfo is the creator-facing payment request. It does not expose creator user IDs or payer identity.
type RequestInfo struct {
	PublicID        uuid.UUID  `json:"public_id"`
	Recipient       string     `json:"recipient"`
	AmountLunas     string     `json:"amount_lunas"`
	AmountNIM       string     `json:"amount_nim"`
	Note            *string    `json:"note"`
	Status          string     `json:"status"`
	ExpiresAt       time.Time  `json:"expires_at"`
	CancelledAt     *time.Time `json:"cancelled_at"`
	PaidAt          *time.Time `json:"paid_at"`
	TransactionHash *string    `json:"transaction_hash,omitempty"`
	Network         string     `json:"network"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// PublicRequestInfo is the payer-facing subset. It intentionally omits creator, payer, and auth data.
type PublicRequestInfo struct {
	PublicID    uuid.UUID `json:"public_id"`
	Recipient   string    `json:"recipient"`
	AmountLunas string    `json:"amount_lunas"`
	AmountNIM   string    `json:"amount_nim"`
	Note        *string   `json:"note"`
	Status      string    `json:"status"`
	ExpiresAt   time.Time `json:"expires_at"`
	Network     string    `json:"network"`
}

// RequestResponse is the response envelope for a single creator-facing request.
type RequestResponse struct {
	Data RequestInfo `json:"data"`
}

// RequestsResponse is the response envelope for a creator list.
type RequestsResponse struct {
	Data []RequestInfo `json:"data"`
}

// PublicRequestResponse is the response envelope for the public read.
type PublicRequestResponse struct {
	Data PublicRequestInfo `json:"data"`
}

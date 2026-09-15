package dto

import (
	"time"

	"github.com/google/uuid"
)

// PurchaseInfo exposes Luna and NIM amounts as strings so JavaScript clients cannot lose integer precision.
type PurchaseInfo struct {
	ID                    uuid.UUID  `json:"id"`
	EventID               uuid.UUID  `json:"event_id"`
	AmountLunas           string     `json:"amount_lunas"`
	AmountNIM             string     `json:"amount_nim"`
	Status                string     `json:"status"`
	TransactionHash       *string    `json:"transaction_hash,omitempty"`
	CapacityHoldExpiresAt *time.Time `json:"capacity_hold_expires_at,omitempty"`
	ConfirmedAt           *time.Time `json:"confirmed_at,omitempty"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
}

// PaymentInstructions contains only server-authoritative public payment values.
type PaymentInstructions struct {
	PurchaseID            uuid.UUID  `json:"purchase_id"`
	Recipient             string     `json:"recipient"`
	AmountLunas           string     `json:"amount_lunas"`
	Network               string     `json:"network"`
	CapacityHoldExpiresAt *time.Time `json:"capacity_hold_expires_at,omitempty"`
}

// SubmitTransactionRequest accepts only the wallet-produced transaction hash.
type SubmitTransactionRequest struct {
	TransactionHash string `json:"transaction_hash"`
}

// PurchaseResponse is the response envelope for purchase operations.
type PurchaseResponse struct {
	Data PurchaseInfo `json:"data"`
}

// PaymentInstructionsResponse is the response envelope for payment instructions.
type PaymentInstructionsResponse struct {
	Data PaymentInstructions `json:"data"`
}

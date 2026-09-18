// Package event defines user-scoped realtime domain events.
package event

import (
	"time"

	"github.com/google/uuid"
)

const (
	TypePaymentRequestSubmitted = "payment_request.submitted"
	TypePaymentRequestVerifying = "payment_request.verifying"
	TypePaymentRequestPaid      = "payment_request.paid"
	TypePaymentRequestFailed    = "payment_request.failed"
	TypePaymentRequestExpired   = "payment_request.expired"
	TypePaymentRequestCancelled = "payment_request.cancelled"

	TypeEventPurchaseSubmitted = "event_purchase.submitted"
	TypeEventPurchaseVerifying = "event_purchase.verifying"
	TypeEventPurchaseConfirmed = "event_purchase.confirmed"
	TypeEventPurchaseFailed    = "event_purchase.failed"
	TypeEventPurchaseExpired   = "event_purchase.expired"
	TypeEventPurchaseCancelled = "event_purchase.cancelled"

	TypeWalletActivityChanged = "wallet.activity_changed"
)

// StatusChanged is published after an authoritative database status transition.
// UserIDs are routing-only and must not be copied into the WebSocket client payload.
type StatusChanged struct {
	Type       string      `json:"type"`
	ResourceID string      `json:"resource_id"`
	Status     string      `json:"status"`
	UserIDs    []uuid.UUID `json:"user_ids,omitempty"`
	OccurredAt time.Time   `json:"occurred_at"`
}

// PaymentRequestEventType maps a persisted payment-request status to a domain event type.
func PaymentRequestEventType(status string) string {
	switch status {
	case "submitted":
		return TypePaymentRequestSubmitted
	case "verifying":
		return TypePaymentRequestVerifying
	case "paid":
		return TypePaymentRequestPaid
	case "failed":
		return TypePaymentRequestFailed
	case "expired":
		return TypePaymentRequestExpired
	case "cancelled":
		return TypePaymentRequestCancelled
	default:
		return ""
	}
}

// EventPurchaseEventType maps a persisted event-purchase status to a domain event type.
func EventPurchaseEventType(status string) string {
	switch status {
	case "submitted":
		return TypeEventPurchaseSubmitted
	case "verifying":
		return TypeEventPurchaseVerifying
	case "confirmed":
		return TypeEventPurchaseConfirmed
	case "failed":
		return TypeEventPurchaseFailed
	case "expired":
		return TypeEventPurchaseExpired
	case "cancelled":
		return TypeEventPurchaseCancelled
	default:
		return ""
	}
}

// IsPaymentDomainType reports whether typeName is a payment-domain realtime event.
func IsPaymentDomainType(typeName string) bool {
	switch typeName {
	case TypePaymentRequestSubmitted, TypePaymentRequestVerifying, TypePaymentRequestPaid,
		TypePaymentRequestFailed, TypePaymentRequestExpired, TypePaymentRequestCancelled,
		TypeEventPurchaseSubmitted, TypeEventPurchaseVerifying, TypeEventPurchaseConfirmed,
		TypeEventPurchaseFailed, TypeEventPurchaseExpired, TypeEventPurchaseCancelled,
		TypeWalletActivityChanged:
		return true
	default:
		return false
	}
}

// UniqueUserIDs returns non-nil unique user IDs.
func UniqueUserIDs(ids ...uuid.UUID) []uuid.UUID {
	seen := make(map[uuid.UUID]struct{}, len(ids))
	out := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if id == uuid.Nil {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

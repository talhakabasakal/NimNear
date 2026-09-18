package event

import (
	"testing"

	"github.com/google/uuid"
)

func TestPaymentRequestEventType(t *testing.T) {
	if PaymentRequestEventType("pending") != "" {
		t.Fatal("pending is not a published payment-request event")
	}
	if PaymentRequestEventType("paid") != TypePaymentRequestPaid {
		t.Fatalf("paid type = %q", PaymentRequestEventType("paid"))
	}
}

func TestEventPurchaseEventType(t *testing.T) {
	if EventPurchaseEventType("pending") != "" {
		t.Fatal("pending is not a published purchase event")
	}
	if EventPurchaseEventType("confirmed") != TypeEventPurchaseConfirmed {
		t.Fatalf("confirmed type = %q", EventPurchaseEventType("confirmed"))
	}
}

func TestUniqueUserIDs(t *testing.T) {
	a := uuid.New()
	b := uuid.New()
	got := UniqueUserIDs(uuid.Nil, a, a, b)
	if len(got) != 2 || got[0] != a || got[1] != b {
		t.Fatalf("got %#v", got)
	}
}

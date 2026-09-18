package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/masterfabric-go/masterfabric/internal/application/nimiqtx/verification"
	"github.com/masterfabric-go/masterfabric/internal/domain/paymentrequest/model"
	realtimeevent "github.com/masterfabric-go/masterfabric/internal/domain/realtime/event"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
	"github.com/masterfabric-go/masterfabric/internal/shared/events"
)

type capturingBus struct {
	events []events.Event
	err    error
}

func (c *capturingBus) Publish(_ context.Context, _ string, event events.Event) error {
	if c.err != nil {
		return c.err
	}
	c.events = append(c.events, event)
	return nil
}

func (c *capturingBus) Subscribe(string, events.Handler) {}
func (c *capturingBus) Close() error                     { return nil }

func (c *capturingBus) types() []string {
	out := make([]string, 0, len(c.events))
	for _, event := range c.events {
		changed, ok := event.(realtimeevent.StatusChanged)
		if !ok {
			continue
		}
		out = append(out, changed.Type)
	}
	return out
}

func TestSubmitTransactionPublishesAfterCommittedTransition(t *testing.T) {
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	creatorID := uuid.New()
	payerID := uuid.New()
	request := pendingRequest(creatorID, now)
	bus := &capturingBus{}
	uc := NewRequestUseCaseWithVerifier(
		&fakeRequestRepository{request: request},
		identityStub{addresses: []string{secondIdentity}},
		DefaultTTL,
		"test-albatross",
		nil,
		&scriptedTransferVerifier{outcome: verification.OutcomeNotFinal},
	).WithEventBus(bus)
	uc.now = func() time.Time { return now }

	result, err := uc.SubmitTransaction(context.Background(), request.PublicID, payerID, strings.Repeat("a", 64))
	if err != nil {
		t.Fatalf("SubmitTransaction: %v", err)
	}
	if result.Data.Status != string(model.StatusVerifying) {
		t.Fatalf("status = %q", result.Data.Status)
	}
	if got := bus.types(); len(got) < 2 || got[0] != realtimeevent.TypePaymentRequestSubmitted || got[1] != realtimeevent.TypePaymentRequestVerifying {
		t.Fatalf("events = %#v", got)
	}
	for _, event := range bus.events {
		changed := event.(realtimeevent.StatusChanged)
		if changed.ResourceID != request.PublicID.String() {
			t.Fatalf("resource id = %q", changed.ResourceID)
		}
		foundCreator, foundPayer := false, false
		for _, id := range changed.UserIDs {
			if id == creatorID {
				foundCreator = true
			}
			if id == payerID {
				foundPayer = true
			}
		}
		if !foundCreator || !foundPayer {
			t.Fatalf("recipients = %#v", changed.UserIDs)
		}
	}
}

func TestPaidTransitionPublishesPaidAndWalletActivity(t *testing.T) {
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	request := pendingRequest(uuid.New(), now)
	bus := &capturingBus{}
	uc := NewRequestUseCaseWithVerifier(
		&fakeRequestRepository{request: request},
		identityStub{addresses: []string{primaryIdentity}},
		DefaultTTL,
		"test-albatross",
		nil,
		&scriptedTransferVerifier{outcome: verification.OutcomeConfirmed},
	).WithEventBus(bus)
	uc.now = func() time.Time { return now }
	if _, err := uc.SubmitTransaction(context.Background(), request.PublicID, uuid.New(), strings.Repeat("a", 64)); err != nil {
		t.Fatalf("SubmitTransaction: %v", err)
	}
	got := bus.types()
	if len(got) < 3 || got[0] != realtimeevent.TypePaymentRequestSubmitted || got[1] != realtimeevent.TypePaymentRequestPaid || got[2] != realtimeevent.TypeWalletActivityChanged {
		t.Fatalf("events = %#v", got)
	}
}

func TestFailedSubmitDoesNotPublish(t *testing.T) {
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	request := pendingRequest(uuid.New(), now)
	bus := &capturingBus{}
	uc := NewRequestUseCaseWithVerifier(
		&fakeRequestRepository{request: request},
		identityStub{addresses: []string{primaryIdentity}},
		DefaultTTL,
		"test-albatross",
		nil,
		&scriptedTransferVerifier{outcome: verification.OutcomeConfirmed},
	).WithEventBus(bus)
	uc.now = func() time.Time { return now }
	if _, err := uc.SubmitTransaction(context.Background(), request.PublicID, uuid.New(), "not-a-hash"); err == nil || !errors.Is(err, domainErr.ErrValidation) {
		t.Fatalf("error = %v", err)
	}
	if len(bus.events) != 0 {
		t.Fatalf("unexpected events %#v", bus.types())
	}
}

func TestCancelPublishesOnlyOnCommittedWrite(t *testing.T) {
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	creatorID := uuid.New()
	request := pendingRequest(creatorID, now)
	bus := &capturingBus{}
	uc := NewRequestUseCase(&fakeRequestRepository{request: request}, identityStub{}, DefaultTTL, "test-albatross", nil).WithEventBus(bus)
	uc.now = func() time.Time { return now }
	if _, err := uc.Cancel(context.Background(), request.PublicID, creatorID); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if got := bus.types(); len(got) != 1 || got[0] != realtimeevent.TypePaymentRequestCancelled {
		t.Fatalf("events = %#v", got)
	}
	if _, err := uc.Cancel(context.Background(), request.PublicID, creatorID); err != nil {
		t.Fatalf("idempotent Cancel: %v", err)
	}
	if got := bus.types(); len(got) != 1 {
		t.Fatalf("duplicate cancel events = %#v", got)
	}
}

func TestExpiryPublishesExpiredEvent(t *testing.T) {
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	request := pendingRequest(uuid.New(), now)
	request.Status = model.StatusSubmitted
	hash := strings.Repeat("a", 64)
	request.TransactionHash = &hash
	payer := uuid.New()
	request.PayerUserID = &payer
	bus := &capturingBus{}
	uc := NewRequestUseCaseWithVerifierAndPolicy(
		&fakeRequestRepository{request: request},
		identityStub{addresses: []string{primaryIdentity}},
		DefaultTTL,
		"test-albatross",
		nil,
		&scriptedTransferVerifier{outcome: verification.OutcomeNotFinal},
		ReconciliationPolicy{Interval: time.Minute, Deadline: time.Hour, BatchSize: 2},
	).WithEventBus(bus)
	stale := now.Add(2 * time.Hour)
	uc.now = func() time.Time { return stale }
	if _, err := uc.Reconcile(context.Background()); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if got := bus.types(); len(got) != 1 || got[0] != realtimeevent.TypePaymentRequestExpired {
		t.Fatalf("events = %#v", got)
	}
}

func TestPublishFailureDoesNotRollBackPaidState(t *testing.T) {
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	request := pendingRequest(uuid.New(), now)
	bus := &capturingBus{err: errors.New("bus unavailable")}
	uc := NewRequestUseCaseWithVerifier(
		&fakeRequestRepository{request: request},
		identityStub{addresses: []string{primaryIdentity}},
		DefaultTTL,
		"test-albatross",
		nil,
		&scriptedTransferVerifier{outcome: verification.OutcomeConfirmed},
	).WithEventBus(bus)
	uc.now = func() time.Time { return now }
	result, err := uc.SubmitTransaction(context.Background(), request.PublicID, uuid.New(), strings.Repeat("a", 64))
	if err != nil {
		t.Fatalf("SubmitTransaction: %v", err)
	}
	if result.Data.Status != string(model.StatusPaid) {
		t.Fatalf("status = %q, payment must remain paid when publish fails", result.Data.Status)
	}
}

func TestDuplicatePublishIsSafe(t *testing.T) {
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	request := pendingRequest(uuid.New(), now)
	bus := &capturingBus{}
	uc := NewRequestUseCaseWithVerifier(
		&fakeRequestRepository{request: request},
		identityStub{addresses: []string{primaryIdentity}},
		DefaultTTL,
		"test-albatross",
		nil,
		&scriptedTransferVerifier{outcome: verification.OutcomeConfirmed},
	).WithEventBus(bus)
	uc.now = func() time.Time { return now }
	hash := strings.Repeat("a", 64)
	payer := uuid.New()
	if _, err := uc.SubmitTransaction(context.Background(), request.PublicID, payer, hash); err != nil {
		t.Fatalf("first submit: %v", err)
	}
	first := len(bus.events)
	if _, err := uc.SubmitTransaction(context.Background(), request.PublicID, payer, hash); err != nil {
		t.Fatalf("duplicate submit: %v", err)
	}
	if len(bus.events) < first {
		t.Fatal("duplicate publish must not lose prior events")
	}
}

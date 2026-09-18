package usecase

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/masterfabric-go/masterfabric/internal/application/eventpurchase/verification"
	"github.com/masterfabric-go/masterfabric/internal/domain/eventpurchase/model"
	realtimeevent "github.com/masterfabric-go/masterfabric/internal/domain/realtime/event"
	"github.com/masterfabric-go/masterfabric/internal/shared/events"
)

type capturingPurchaseBus struct {
	events []events.Event
}

func (c *capturingPurchaseBus) Publish(_ context.Context, _ string, event events.Event) error {
	c.events = append(c.events, event)
	return nil
}
func (c *capturingPurchaseBus) Subscribe(string, events.Handler) {}
func (c *capturingPurchaseBus) Close() error                     { return nil }

func (c *capturingPurchaseBus) types() []string {
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

func TestPurchaseSubmitPublishesAuthorizedOwnerEvents(t *testing.T) {
	now := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)
	purchase := &model.Purchase{ID: uuid.New(), EventID: uuid.New(), UserID: uuid.New(), AmountLunas: 250000, Status: model.StatusPending, CreatedAt: now, UpdatedAt: now}
	bus := &capturingPurchaseBus{}
	uc := NewPurchaseUseCaseWithVerifier(&fakePurchaseRepository{purchase: purchase}, time.Minute, &scriptedVerifier{outcome: verification.OutcomeNotFinal}, "merchant", "MainAlbatross").WithIdentities(identityStub{addresses: []string{"NQ46 KLJE 5TMF 4Y1A 1255 CJHJ YG1S H0NU T604"}}).WithEventBus(bus)
	uc.now = func() time.Time { return now }
	result, err := uc.SubmitTransaction(context.Background(), purchase.ID, purchase.UserID, strings.Repeat("a", 64))
	if err != nil {
		t.Fatalf("SubmitTransaction: %v", err)
	}
	if result.Data.Status != string(model.StatusVerifying) {
		t.Fatalf("status = %q", result.Data.Status)
	}
	if got := bus.types(); len(got) < 2 || got[0] != realtimeevent.TypeEventPurchaseSubmitted || got[1] != realtimeevent.TypeEventPurchaseVerifying {
		t.Fatalf("events = %#v", got)
	}
	changed := bus.events[0].(realtimeevent.StatusChanged)
	if changed.ResourceID != purchase.ID.String() || len(changed.UserIDs) != 1 || changed.UserIDs[0] != purchase.UserID {
		t.Fatalf("event = %#v", changed)
	}
}

func TestPurchaseConfirmedPublishesWalletActivity(t *testing.T) {
	purchase := &model.Purchase{ID: uuid.New(), EventID: uuid.New(), UserID: uuid.New(), AmountLunas: 250000, Status: model.StatusPending}
	bus := &capturingPurchaseBus{}
	uc := NewPurchaseUseCaseWithVerifier(&fakePurchaseRepository{purchase: purchase}, time.Minute, &scriptedVerifier{outcome: verification.OutcomeConfirmed}, "merchant", "MainAlbatross").WithIdentities(identityStub{addresses: []string{"NQ46 KLJE 5TMF 4Y1A 1255 CJHJ YG1S H0NU T604"}}).WithEventBus(bus)
	if _, err := uc.SubmitTransaction(context.Background(), purchase.ID, purchase.UserID, strings.Repeat("a", 64)); err != nil {
		t.Fatalf("SubmitTransaction: %v", err)
	}
	got := bus.types()
	if len(got) < 3 || got[1] != realtimeevent.TypeEventPurchaseConfirmed || got[2] != realtimeevent.TypeWalletActivityChanged {
		t.Fatalf("events = %#v", got)
	}
}

func TestPurchaseInvalidHashDoesNotPublish(t *testing.T) {
	purchase := &model.Purchase{ID: uuid.New(), EventID: uuid.New(), UserID: uuid.New(), AmountLunas: 250000, Status: model.StatusPending}
	bus := &capturingPurchaseBus{}
	uc := NewPurchaseUseCase(&fakePurchaseRepository{purchase: purchase}, time.Minute).WithEventBus(bus)
	if _, err := uc.SubmitTransaction(context.Background(), purchase.ID, purchase.UserID, "not-a-hash"); err == nil {
		t.Fatal("expected validation error")
	}
	if len(bus.events) != 0 {
		t.Fatalf("unexpected events %#v", bus.types())
	}
}

func TestRecoverExpiredPublishesAfterConfirm(t *testing.T) {
	hash := strings.Repeat("a", 64)
	purchase := &model.Purchase{ID: uuid.New(), EventID: uuid.New(), UserID: uuid.New(), AmountLunas: 250000, Status: model.StatusExpired, TransactionHash: &hash}
	bus := &capturingPurchaseBus{}
	uc := NewPurchaseUseCaseWithVerifier(&fakePurchaseRepository{purchase: purchase}, time.Minute, &scriptedVerifier{outcome: verification.OutcomeConfirmed}, "merchant", "MainAlbatross").WithEventBus(bus)
	result, err := uc.RecoverExpired(context.Background(), purchase.ID, purchase.UserID)
	if err != nil {
		t.Fatalf("RecoverExpired: %v", err)
	}
	if result.Data.Status != string(model.StatusConfirmed) {
		t.Fatalf("status = %q", result.Data.Status)
	}
	got := bus.types()
	if len(got) < 2 || got[0] != realtimeevent.TypeEventPurchaseConfirmed || got[1] != realtimeevent.TypeWalletActivityChanged {
		t.Fatalf("events = %#v", got)
	}
}

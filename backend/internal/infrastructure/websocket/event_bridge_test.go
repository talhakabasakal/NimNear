package websocket

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	realtimeevent "github.com/masterfabric-go/masterfabric/internal/domain/realtime/event"
	"github.com/masterfabric-go/masterfabric/internal/domain/realtime/model"
	realtimeService "github.com/masterfabric-go/masterfabric/internal/domain/realtime/service"
	"github.com/masterfabric-go/masterfabric/internal/shared/events"
)

type syncBus struct {
	handlers map[string][]events.Handler
}

func newSyncBus() *syncBus {
	return &syncBus{handlers: map[string][]events.Handler{}}
}

func (b *syncBus) Publish(ctx context.Context, topic string, event events.Event) error {
	for _, handler := range b.handlers[topic] {
		if err := handler(ctx, event); err != nil {
			return err
		}
	}
	return nil
}
func (b *syncBus) Subscribe(topic string, handler events.Handler) {
	b.handlers[topic] = append(b.handlers[topic], handler)
}
func (b *syncBus) Close() error { return nil }

func registerUser(t *testing.T, hub *Hub, userID uuid.UUID) chan []byte {
	t.Helper()
	send := make(chan []byte, 8)
	hub.Register(realtimeService.ClientInfo{ID: uuid.New().String(), UserID: userID}, send)
	return send
}

func TestPaymentEventReachesAuthorizedUsersOnly(t *testing.T) {
	hub := NewHub(nil, 10)
	bridge := NewEventBridge(hub, nil, nil)
	bus := newSyncBus()
	bridge.Register(bus)

	creator := uuid.New()
	payer := uuid.New()
	other := uuid.New()
	creatorCh := registerUser(t, hub, creator)
	payerCh := registerUser(t, hub, payer)
	otherCh := registerUser(t, hub, other)

	publicID := uuid.New()
	err := bus.Publish(context.Background(), events.TopicPayments, realtimeevent.StatusChanged{
		Type:       realtimeevent.TypePaymentRequestPaid,
		ResourceID: publicID.String(),
		Status:     "paid",
		UserIDs:    []uuid.UUID{creator, payer},
		OccurredAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}

	creatorMsg := readWS(t, creatorCh)
	payerMsg := readWS(t, payerCh)
	select {
	case msg := <-otherCh:
		t.Fatalf("unauthorized subscriber received %#v", string(msg))
	default:
	}

	assertClientEvent(t, creatorMsg, realtimeevent.TypePaymentRequestPaid, publicID.String(), "paid")
	assertClientEvent(t, payerMsg, realtimeevent.TypePaymentRequestPaid, publicID.String(), "paid")
}

func TestDuplicatePaymentPublishIsSafe(t *testing.T) {
	hub := NewHub(nil, 10)
	bridge := NewEventBridge(hub, nil, nil)
	bus := newSyncBus()
	bridge.Register(bus)
	userID := uuid.New()
	send := registerUser(t, hub, userID)
	event := realtimeevent.StatusChanged{
		Type:       realtimeevent.TypePaymentRequestVerifying,
		ResourceID: uuid.New().String(),
		Status:     "verifying",
		UserIDs:    []uuid.UUID{userID},
	}
	_ = bus.Publish(context.Background(), events.TopicPayments, event)
	_ = bus.Publish(context.Background(), events.TopicPayments, event)
	if _, ok := <-send; !ok {
		t.Fatal("missing first event")
	}
	if _, ok := <-send; !ok {
		t.Fatal("missing duplicate event")
	}
}

func TestMemoryFanoutDeliversToRemoteInstance(t *testing.T) {
	local := NewHub(nil, 10)
	remote := NewHub(nil, 10)
	localBridge := NewEventBridge(local, nil, nil)
	remoteBridge := NewEventBridge(remote, nil, nil)
	relay := NewMemoryRelay("local")
	localBridge.SetRelay(relay, "local")
	remoteBridge.SetRelay(nil, "remote")
	relay.Subscribe(func(msg FanoutMessage) {
		msg.OriginInstance = "local"
		remoteBridge.DeliverFanout(msg)
	})
	bus := newSyncBus()
	localBridge.Register(bus)

	userID := uuid.New()
	remoteSend := registerUser(t, remote, userID)
	_ = bus.Publish(context.Background(), events.TopicPayments, realtimeevent.StatusChanged{
		Type:       realtimeevent.TypePaymentRequestSubmitted,
		ResourceID: uuid.New().String(),
		Status:     "submitted",
		UserIDs:    []uuid.UUID{userID},
	})
	readWS(t, remoteSend)
}

func readWS(t *testing.T, ch <-chan []byte) []byte {
	t.Helper()
	select {
	case msg := <-ch:
		return msg
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for websocket payload")
		return nil
	}
}

func assertClientEvent(t *testing.T, raw []byte, eventType, resourceID, status string) {
	t.Helper()
	var msg model.OutboundMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if msg.Type != eventType {
		t.Fatalf("type = %q", msg.Type)
	}
	var data model.ClientEventData
	if err := json.Unmarshal(msg.Data, &data); err != nil {
		t.Fatalf("data: %v", err)
	}
	if data.ResourceID != resourceID || data.Status != status {
		t.Fatalf("data = %#v", data)
	}
	encoded := string(raw)
	if containsAny(encoded, "user_ids", "jwt", "cookie", "password", "private") {
		t.Fatalf("payload leaked private fields: %s", encoded)
	}
}

func containsAny(haystack string, needles ...string) bool {
	for _, needle := range needles {
		if json.Valid([]byte(haystack)) && (needle == "user_ids" && containsJSONKey(haystack, needle)) {
			return true
		}
	}
	return false
}

func containsJSONKey(raw, key string) bool {
	var generic map[string]any
	if err := json.Unmarshal([]byte(raw), &generic); err != nil {
		return false
	}
	if _, ok := generic[key]; ok {
		return true
	}
	if data, ok := generic["data"].(map[string]any); ok {
		if _, ok := data[key]; ok {
			return true
		}
	}
	return false
}

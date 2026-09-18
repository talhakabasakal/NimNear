package websocket

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const redisRealtimeChannel = "nimnear:realtime:ws"

// FanoutMessage is an internal multi-instance routing envelope. It is never sent to browsers.
type FanoutMessage struct {
	OriginInstance string          `json:"origin_instance"`
	UserIDs        []string        `json:"user_ids"`
	Payload        json.RawMessage `json:"payload"`
}

// Relay publishes WebSocket payloads to other backend instances.
type Relay interface {
	Publish(ctx context.Context, userIDs []uuid.UUID, payload []byte) error
	Listen(ctx context.Context, handler func(FanoutMessage))
}

// RedisRelay fans out user-scoped WebSocket payloads with Redis pub/sub.
type RedisRelay struct {
	client     *redis.Client
	instanceID string
	logger     *slog.Logger
}

// NewRedisRelay returns nil when Redis is unavailable so callers can skip fan-out.
func NewRedisRelay(client *redis.Client, instanceID string, logger *slog.Logger) *RedisRelay {
	if client == nil || instanceID == "" {
		return nil
	}
	return &RedisRelay{client: client, instanceID: instanceID, logger: logger}
}

// Publish sends a payload to other instances. Failures are best-effort.
func (r *RedisRelay) Publish(ctx context.Context, userIDs []uuid.UUID, payload []byte) error {
	if r == nil || r.client == nil {
		return nil
	}
	ids := make([]string, 0, len(userIDs))
	for _, id := range userIDs {
		if id != uuid.Nil {
			ids = append(ids, id.String())
		}
	}
	body, err := json.Marshal(FanoutMessage{
		OriginInstance: r.instanceID,
		UserIDs:        ids,
		Payload:        payload,
	})
	if err != nil {
		observeFanoutFailure()
		return err
	}
	if err := r.client.Publish(ctx, redisRealtimeChannel, body).Err(); err != nil {
		observeFanoutFailure()
		if r.logger != nil {
			r.logger.Error("realtime redis fanout publish failed", "error", err)
		}
		return err
	}
	return nil
}

// Listen subscribes for remote instance payloads until ctx is cancelled.
func (r *RedisRelay) Listen(ctx context.Context, handler func(FanoutMessage)) {
	if r == nil || r.client == nil || handler == nil {
		return
	}
	pubsub := r.client.Subscribe(ctx, redisRealtimeChannel)
	defer pubsub.Close()
	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			if msg == nil {
				continue
			}
			var envelope FanoutMessage
			if err := json.Unmarshal([]byte(msg.Payload), &envelope); err != nil {
				if r.logger != nil {
					r.logger.Error("realtime redis fanout decode failed", "error", err)
				}
				continue
			}
			if envelope.OriginInstance == r.instanceID {
				continue
			}
			handler(envelope)
		}
	}
}

// MemoryRelay is a process-local fan-out used by tests.
type MemoryRelay struct {
	instanceID string
	handlers   []func(FanoutMessage)
}

// NewMemoryRelay creates an in-process relay.
func NewMemoryRelay(instanceID string) *MemoryRelay {
	return &MemoryRelay{instanceID: instanceID}
}

// Subscribe registers a local handler.
func (m *MemoryRelay) Subscribe(handler func(FanoutMessage)) {
	if m == nil || handler == nil {
		return
	}
	m.handlers = append(m.handlers, handler)
}

// Publish implements Relay.
func (m *MemoryRelay) Publish(_ context.Context, userIDs []uuid.UUID, payload []byte) error {
	if m == nil {
		return nil
	}
	ids := make([]string, 0, len(userIDs))
	for _, id := range userIDs {
		if id != uuid.Nil {
			ids = append(ids, id.String())
		}
	}
	msg := FanoutMessage{OriginInstance: m.instanceID, UserIDs: ids, Payload: payload}
	for _, handler := range m.handlers {
		handler(msg)
	}
	return nil
}

// Listen is a no-op; tests call Subscribe directly.
func (m *MemoryRelay) Listen(context.Context, func(FanoutMessage)) {}

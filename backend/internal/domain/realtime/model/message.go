package model

import (
	"encoding/json"
	"time"
)

// Client actions.
const (
	ActionSubscribe   = "subscribe"
	ActionUnsubscribe = "unsubscribe"
	ActionPing        = "ping"
)

// Server message types.
const (
	TypePong       = "pong"
	TypeSubscribed = "subscribed"
	TypeError      = "error"
)

// InboundMessage is a JSON message from the client.
type InboundMessage struct {
	Action  string `json:"action"`
	Channel string `json:"channel,omitempty"`
}

// OutboundMessage is a JSON message pushed to the client.
type OutboundMessage struct {
	Type           string          `json:"type"`
	Topic          string          `json:"topic,omitempty"`
	OrganizationID string          `json:"organization_id,omitempty"`
	AppID          string          `json:"app_id,omitempty"`
	Channel        string          `json:"channel,omitempty"`
	Message        string          `json:"message,omitempty"`
	Data           json.RawMessage `json:"data,omitempty"`
	Timestamp      time.Time       `json:"timestamp,omitempty"`
}

// ClientEventData is the browser-visible payload for payment-domain events.
type ClientEventData struct {
	ResourceID string `json:"resource_id"`
	Status     string `json:"status"`
}

// NewEventMessage builds a domain event push payload.
func NewEventMessage(eventType, topic, orgID, appID string, data json.RawMessage) OutboundMessage {
	return OutboundMessage{
		Type:           eventType,
		Topic:          topic,
		OrganizationID: orgID,
		AppID:          appID,
		Data:           data,
		Timestamp:      time.Now().UTC(),
	}
}

// NewUserEventMessage builds a user-scoped event without tenant identifiers.
func NewUserEventMessage(eventType, topic string, data ClientEventData) (OutboundMessage, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return OutboundMessage{}, err
	}
	return OutboundMessage{
		Type:      eventType,
		Topic:     topic,
		Data:      raw,
		Timestamp: time.Now().UTC(),
	}, nil
}

// NewControlMessage builds a control response (pong, subscribed, error).
func NewControlMessage(msgType, channel, message string) OutboundMessage {
	return OutboundMessage{
		Type:      msgType,
		Channel:   channel,
		Message:   message,
		Timestamp: time.Now().UTC(),
	}
}

package events

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

type Payload interface {
	EventType() string
}

type Envelope struct {
	EventID      string          `json:"eventId"`
	EventType    string          `json:"eventType"`
	EventVersion int             `json:"eventVersion"`
	OccurredAt   time.Time       `json:"occurredAt"`
	Payload      json.RawMessage `json:"payload"`
}

func Wrap(id string, at time.Time, payload Payload) Envelope {
	raw, _ := json.Marshal(payload)
	return Envelope{
		EventID:      id,
		EventType:    payload.EventType(),
		EventVersion: 1,
		OccurredAt:   at.UTC(),
		Payload:      raw,
	}
}

func Encode(env Envelope) ([]byte, error) {
	return json.Marshal(env)
}

var ErrTypeMismatch = errors.New("events: payload type does not match envelope eventType")

func Decode(raw []byte) (Envelope, error) {
	var env Envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return Envelope{}, fmt.Errorf("events: decode envelope: %w", err)
	}
	return env, nil
}

func PayloadAs[T Payload](env Envelope) (T, error) {
	var out T
	if out.EventType() != env.EventType {
		return out, fmt.Errorf("%w: want %s, got %s", ErrTypeMismatch, out.EventType(), env.EventType)
	}
	if err := json.Unmarshal(env.Payload, &out); err != nil {
		return out, fmt.Errorf("events: decode payload %s: %w", env.EventType, err)
	}
	return out, nil
}

type PendingEvent struct {
	ID       int64
	Envelope Envelope
}

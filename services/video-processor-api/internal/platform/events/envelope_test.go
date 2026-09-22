package events_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/ivanzao/fiap-video-processor/services/video-processor-api/internal/platform/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncodeWrapsPayloadInEnvelopeWithCamelCaseFields(t *testing.T) {
	at := time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC)
	payload := events.VideoProcessingRequested{
		RequestID: "req-1",
		VideoID:   "vid-1",
		UserID:    "usr-1",
		UserEmail: "ana@example.com",
		ObjectKey: "uploads/vid-1.mp4",
		Filename:  "ferias.mp4",
	}

	raw, err := events.Encode(events.Wrap("evt-1", at, payload))
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal(raw, &got))
	assert.Equal(t, "evt-1", got["eventId"])
	assert.Equal(t, "VideoProcessingRequested", got["eventType"])
	assert.Equal(t, float64(1), got["eventVersion"])
	assert.Equal(t, "2026-09-13T10:00:00Z", got["occurredAt"])
	assert.Equal(t, "ana@example.com", got["payload"].(map[string]any)["userEmail"])
}

func TestDecodeRoundTripsATypedPayload(t *testing.T) {
	at := time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC)
	raw, err := events.Encode(events.Wrap("evt-2", at, events.VideoProcessingCompleted{
		RequestID: "req-1", ZipKey: "results/req-1.zip", FrameCount: 42, DurationMs: 1500,
	}))
	require.NoError(t, err)

	env, err := events.Decode(raw)
	require.NoError(t, err)
	assert.Equal(t, "evt-2", env.EventID)
	assert.Equal(t, events.TypeVideoProcessingCompleted, env.EventType)
	assert.True(t, at.Equal(env.OccurredAt))

	got, err := events.PayloadAs[events.VideoProcessingCompleted](env)
	require.NoError(t, err)
	assert.Equal(t, 42, got.FrameCount)
	assert.Equal(t, "results/req-1.zip", got.ZipKey)
}

func TestPayloadAsRejectsMismatchedEventType(t *testing.T) {
	env := events.Wrap("evt-3", time.Now(), events.VideoProcessingStarted{RequestID: "req-1", Attempt: 1})

	_, err := events.PayloadAs[events.VideoProcessingCompleted](env)

	assert.ErrorIs(t, err, events.ErrTypeMismatch)
}

func TestDecodeRejectsMalformedJSON(t *testing.T) {
	_, err := events.Decode([]byte(`{"eventId": `))

	assert.Error(t, err)
}

func TestMessageAttributesCarryEventTypeAndTraceContext(t *testing.T) {
	env := events.Wrap("evt-4", time.Now(), events.VideoProcessingFailed{RequestID: "req-1", Reason: "boom", Retryable: true, Attempt: 2})

	attrs := events.MessageAttributes(env, "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")

	assert.Equal(t, map[string]string{
		"eventType":   "VideoProcessingFailed",
		"traceparent": "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
	}, attrs)
}

func TestMessageAttributesOmitEmptyTraceContext(t *testing.T) {
	env := events.Wrap("evt-5", time.Now(), events.VideoProcessingStarted{RequestID: "req-1", Attempt: 1})

	attrs := events.MessageAttributes(env, "")

	assert.Equal(t, map[string]string{"eventType": "VideoProcessingStarted"}, attrs)
}

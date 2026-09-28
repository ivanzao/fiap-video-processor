package sns_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ivanzao/fiap-video-processor/services/video-processor-worker/internal/adapters/outbound/sns"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-worker/internal/domain/video"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-worker/internal/platform/events"
)

const topicARN = "arn:aws:sns:us-east-1:000000000000:video-processor-worker-events"

var now = time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)

type clock struct{}

func (clock) Now() time.Time { return now }

type ids struct{}

func (ids) NewID() string { return "evt-1" }

type fakeTopic struct {
	mu       sync.Mutex
	status   int
	received []url.Values
}

func (f *fakeTopic) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	f.mu.Lock()
	f.received = append(f.received, r.PostForm)
	f.mu.Unlock()
	w.Header().Set("Content-Type", "text/xml")
	if f.status != 0 {
		w.WriteHeader(f.status)
		_, _ = w.Write([]byte(`<ErrorResponse><Error><Type>Sender</Type><Code>NotFound</Code><Message>Topic does not exist</Message></Error><RequestId>r</RequestId></ErrorResponse>`))
		return
	}
	_, _ = w.Write([]byte(`<PublishResponse xmlns="http://sns.amazonaws.com/doc/2010-03-31/"><PublishResult><MessageId>m-1</MessageId></PublishResult><ResponseMetadata><RequestId>r</RequestId></ResponseMetadata></PublishResponse>`))
}

func connect(t *testing.T, topic *fakeTopic) *sns.Publisher {
	t.Helper()
	server := httptest.NewServer(topic)
	t.Cleanup(server.Close)
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	p, err := sns.Connect(t.Context(), sns.Config{Region: "us-east-1", Endpoint: server.URL, TopicARN: topicARN}, ids{}, clock{})
	require.NoError(t, err)
	return p
}

func TestPublisherWrapsEachOutcomeInAnEnvelope(t *testing.T) {
	topic := &fakeTopic{}
	p := connect(t, topic)
	ctx := t.Context()

	require.NoError(t, p.PublishStarted(ctx, video.ProcessingStarted{RequestID: "req-1", Attempt: 2}))
	require.NoError(t, p.PublishCompleted(ctx, video.ProcessingCompleted{RequestID: "req-1", ZipKey: "results/req-1.zip", FrameCount: 9, Duration: 1500 * time.Millisecond}))
	require.NoError(t, p.PublishFailed(ctx, video.ProcessingFailed{RequestID: "req-1", Reason: "unprocessable video", Retryable: false, Attempt: 3}))

	require.Len(t, topic.received, 3)
	var envs []events.Envelope
	for _, form := range topic.received {
		assert.Equal(t, "Publish", form.Get("Action"))
		assert.Equal(t, topicARN, form.Get("TopicArn"))
		env, err := events.Decode([]byte(form.Get("Message")))
		require.NoError(t, err)
		assert.Equal(t, "evt-1", env.EventID)
		assert.Equal(t, now, env.OccurredAt)
		assert.Equal(t, "eventType", form.Get("MessageAttributes.entry.1.Name"))
		assert.Equal(t, env.EventType, form.Get("MessageAttributes.entry.1.Value.StringValue"))
		envs = append(envs, env)
	}
	started, err := events.PayloadAs[events.VideoProcessingStarted](envs[0])
	require.NoError(t, err)
	assert.Equal(t, events.VideoProcessingStarted{RequestID: "req-1", Attempt: 2}, started)
	completed, err := events.PayloadAs[events.VideoProcessingCompleted](envs[1])
	require.NoError(t, err)
	assert.Equal(t, events.VideoProcessingCompleted{RequestID: "req-1", ZipKey: "results/req-1.zip", FrameCount: 9, DurationMs: 1500}, completed)
	failed, err := events.PayloadAs[events.VideoProcessingFailed](envs[2])
	require.NoError(t, err)
	assert.Equal(t, events.VideoProcessingFailed{RequestID: "req-1", Reason: "unprocessable video", Retryable: false, Attempt: 3}, failed)
}

func TestPublisherReportsRejectedPublishes(t *testing.T) {
	p := connect(t, &fakeTopic{status: http.StatusNotFound})

	err := p.PublishStarted(t.Context(), video.ProcessingStarted{RequestID: "req-1", Attempt: 1})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "sns: publish VideoProcessingStarted")
}

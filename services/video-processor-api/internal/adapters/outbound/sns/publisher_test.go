package sns_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ivanzao/fiap-video-processor/services/video-processor-api/internal/adapters/outbound/sns"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-api/internal/platform/events"
)

const topicARN = "arn:aws:sns:us-east-1:000000000000:video-processor-api-events"

type fakeTopic struct {
	status   int
	received []url.Values
}

func (f *fakeTopic) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	f.received = append(f.received, r.PostForm)
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
	p, err := sns.Connect(t.Context(), sns.Config{Region: "us-east-1", Endpoint: server.URL, TopicARN: topicARN})
	require.NoError(t, err)
	return p
}

func requested() events.Envelope {
	return events.Wrap("evt-1", time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC), events.VideoProcessingRequested{
		RequestID: "req-1", VideoID: "vid-1", UserID: "usr-ana", UserEmail: "ana@example.com",
		ObjectKey: "uploads/usr-ana/vid-1.mp4", Filename: "ferias.mp4",
	})
}

func TestPublishSendsTheEnvelopeWithItsEventType(t *testing.T) {
	topic := &fakeTopic{}
	env := requested()

	require.NoError(t, connect(t, topic).Publish(t.Context(), env))

	require.Len(t, topic.received, 1)
	form := topic.received[0]
	assert.Equal(t, "Publish", form.Get("Action"))
	assert.Equal(t, topicARN, form.Get("TopicArn"))
	sent, err := events.Decode([]byte(form.Get("Message")))
	require.NoError(t, err)
	assert.Equal(t, env, sent)
	assert.Equal(t, "eventType", form.Get("MessageAttributes.entry.1.Name"))
	assert.Equal(t, events.TypeVideoProcessingRequested, form.Get("MessageAttributes.entry.1.Value.StringValue"))
}

func TestPublishReportsRejectedPublishes(t *testing.T) {
	err := connect(t, &fakeTopic{status: http.StatusNotFound}).Publish(t.Context(), requested())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "sns: publish VideoProcessingRequested")
}

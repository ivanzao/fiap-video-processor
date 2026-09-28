package mail_test

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ivanzao/fiap-video-processor/services/video-processor-worker/internal/adapters/outbound/mail"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-worker/internal/domain/video"
)

func TestComposeDescribesSuccessAndFailure(t *testing.T) {
	ok := mail.Compose(video.Notification{To: "ana@example.com", Filename: "ferias.mp4", Outcome: video.OutcomeCompleted, FrameCount: 42, RequestID: "req-1"})
	assert.Equal(t, "Seu vídeo ferias.mp4 foi processado", ok.Subject)
	assert.Contains(t, ok.Text, "42 frames")

	failed := mail.Compose(video.Notification{To: "ana@example.com", Filename: "ferias.mp4", Outcome: video.OutcomeFailed, Reason: "unprocessable video: corrupt", RequestID: "req-1"})
	assert.Equal(t, "Falha ao processar o vídeo ferias.mp4", failed.Subject)
	assert.Contains(t, failed.Text, "unprocessable video: corrupt")
}

func TestMailerSendNotifierPostsTheMessageWithBearerToken(t *testing.T) {
	var got map[string]any
	var auth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()
	n := mail.NewMailerSendNotifier(server.Client(), server.URL, "secret", "noreply@fiapx.example")

	err := n.Send(t.Context(), video.Notification{To: "ana@example.com", Filename: "ferias.mp4", Outcome: video.OutcomeCompleted, FrameCount: 3})

	require.NoError(t, err)
	assert.Equal(t, "Bearer secret", auth)
	assert.Equal(t, "noreply@fiapx.example", got["from"].(map[string]any)["email"])
	assert.Equal(t, "ana@example.com", got["to"].([]any)[0].(map[string]any)["email"])
}

func TestMailerSendNotifierReportsRejectedRequests(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "quota exceeded", http.StatusTooManyRequests)
	}))
	defer server.Close()
	n := mail.NewMailerSendNotifier(server.Client(), server.URL, "secret", "noreply@fiapx.example")

	err := n.Send(t.Context(), video.Notification{To: "ana@example.com", Outcome: video.OutcomeFailed})

	assert.ErrorContains(t, err, "429")
}

func TestSMTPNotifierReportsUnreachableServers(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := l.Addr().String()
	require.NoError(t, l.Close())

	err = mail.NewSMTPNotifier(addr, "noreply@fiapx.example").Send(t.Context(), video.Notification{To: "ana@example.com", Outcome: video.OutcomeCompleted})

	assert.ErrorContains(t, err, "smtp: send")
}

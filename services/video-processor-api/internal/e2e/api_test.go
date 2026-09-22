//go:build integration

package e2e_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ivanzao/fiap-video-processor/services/video-processor-api/internal/adapters/inbound/httpapi"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-api/internal/adapters/inbound/sqs"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-api/internal/adapters/outbound/outbox"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-api/internal/adapters/outbound/postgres"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-api/internal/adapters/outbound/s3"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-api/internal/adapters/outbound/sns"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-api/internal/domain/video"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-api/internal/platform/events"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-api/internal/platform/system"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-api/internal/testinfra"
)

const bucket = "video-processor"

type stack struct {
	server      *httptest.Server
	aws         testinfra.AWS
	repo        *postgres.VideoRepository
	relay       *outbox.Relay
	consumer    *sqs.Consumer
	workerInbox string
	workerTopic string
}

func startStack(t *testing.T) stack {
	t.Helper()
	pool := testinfra.StartPostgres(t)
	cloud := testinfra.StartLocalStack(t)
	cloud.CreateBucket(t, bucket)
	apiTopic := cloud.CreateTopic(t, "video-processor-api-events")
	workerTopic := cloud.CreateTopic(t, "video-processor-worker-events")
	workerInbox := cloud.CreateQueueSubscribedTo(t, "video-processor-worker-inbox", apiTopic)
	apiInbox := cloud.CreateQueueSubscribedTo(t, "video-processor-api-inbox", workerTopic)

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	repo := postgres.NewVideoRepository(pool)
	svc := video.NewService(repo, s3.NewStore(cloud.S3, cloud.S3, bucket), system.Clock{}, system.UUIDGenerator{}, video.Config{
		MaxUploadBytes: 500 * 1024 * 1024, UploadTTL: 5 * time.Minute, DownloadTTL: 5 * time.Minute,
	})
	server := httptest.NewServer(httpapi.NewRouter(svc, nil))
	t.Cleanup(server.Close)
	return stack{
		server:      server,
		aws:         cloud,
		repo:        repo,
		relay:       outbox.NewRelay(postgres.NewOutbox(pool), sns.NewPublisher(cloud.SNS, apiTopic), log, time.Second, 10),
		consumer:    sqs.NewConsumer(cloud.SQS, apiInbox, svc, log),
		workerInbox: workerInbox,
		workerTopic: workerTopic,
	}
}

func (s stack) call(t *testing.T, method, path, body string) (int, map[string]any) {
	t.Helper()
	req, err := http.NewRequest(method, s.server.URL+path, strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-Id", "usr-ana")
	req.Header.Set("X-User-Email", "ana@example.com")
	res, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = res.Body.Close() }()
	var out map[string]any
	_ = json.NewDecoder(res.Body).Decode(&out)
	return res.StatusCode, out
}

func TestCreateProcessAndDownloadEndToEnd(t *testing.T) {
	s := startStack(t)
	ctx := t.Context()
	content := bytes.Repeat([]byte("frame"), 200)

	status, ticket := s.call(t, http.MethodPost, "/v1/videos", `{"filename":"ferias.mp4","contentType":"video/mp4","sizeBytes":1000}`)
	require.Equal(t, http.StatusCreated, status, ticket)
	videoID := ticket["videoId"].(string)

	put, err := http.NewRequest(http.MethodPut, ticket["uploadUrl"].(string), bytes.NewReader(content))
	require.NoError(t, err)
	for name, value := range ticket["uploadHeaders"].(map[string]any) {
		put.Header.Set(name, value.(string))
	}
	put.ContentLength = int64(len(content))
	putRes, err := http.DefaultClient.Do(put)
	require.NoError(t, err)
	_ = putRes.Body.Close()
	require.Equal(t, http.StatusOK, putRes.StatusCode, "browser-style PUT to the presigned URL")

	status, confirmed := s.call(t, http.MethodPost, "/v1/videos/"+videoID+"/process", "")
	require.Equal(t, http.StatusAccepted, status, confirmed)
	assert.Equal(t, "PENDING", confirmed["status"])
	assert.Equal(t, "ferias.mp4", confirmed["filename"])
	requestID := confirmed["requestId"].(string)

	require.NoError(t, s.relay.Drain(ctx))
	bodies := s.aws.Receive(t, s.workerInbox)
	require.Len(t, bodies, 1, "exactly one VideoProcessingRequested reaches the worker inbox")
	env, err := events.Decode([]byte(bodies[0]))
	require.NoError(t, err)
	requested, err := events.PayloadAs[events.VideoProcessingRequested](env)
	require.NoError(t, err)
	assert.Equal(t, requestID, requested.RequestID)
	assert.Equal(t, "ana@example.com", requested.UserEmail)
	assert.Equal(t, "uploads/usr-ana/"+videoID+".mp4", requested.ObjectKey)
	require.NoError(t, s.relay.Drain(ctx))
	assert.Empty(t, s.aws.Receive(t, s.workerInbox), "outbox rows are published once")

	status, listed := s.call(t, http.MethodGet, "/v1/videos", "")
	require.Equal(t, http.StatusOK, status)
	require.Len(t, listed["items"].([]any), 1)

	status, _ = s.call(t, http.MethodGet, "/v1/videos/"+videoID+"/download", "")
	assert.Equal(t, http.StatusConflict, status, "download before completion is refused")

	zipKey := "results/" + requestID + ".zip"
	_, err = s.aws.S3.PutObject(ctx, &awss3.PutObjectInput{Bucket: aws.String(bucket), Key: aws.String(zipKey), Body: bytes.NewReader([]byte("PK"))})
	require.NoError(t, err)
	publish := func(id string, p events.Payload) {
		s.aws.PublishJSON(t, s.workerTopic, events.Wrap(id, time.Now(), p))
	}
	publish(uuid.NewString(), events.VideoProcessingStarted{RequestID: requestID, Attempt: 1})
	publish(uuid.NewString(), events.VideoProcessingCompleted{RequestID: requestID, ZipKey: zipKey, FrameCount: 12, DurationMs: 900})
	consumerCtx, stop := context.WithCancel(ctx)
	go s.consumer.Run(consumerCtx)
	t.Cleanup(stop)

	require.Eventually(t, func() bool {
		_, listed := s.call(t, http.MethodGet, "/v1/videos", "")
		item := listed["items"].([]any)[0].(map[string]any)
		return item["status"] == "COMPLETED"
	}, 20*time.Second, 250*time.Millisecond)
	stop()

	_, listed = s.call(t, http.MethodGet, "/v1/videos", "")
	item := listed["items"].([]any)[0].(map[string]any)
	assert.Equal(t, float64(1), item["attempts"])
	assert.Equal(t, float64(12), item["result"].(map[string]any)["frameCount"])

	status, download := s.call(t, http.MethodGet, "/v1/videos/"+videoID+"/download", "")
	require.Equal(t, http.StatusOK, status, download)
	res, err := http.Get(download["downloadUrl"].(string))
	require.NoError(t, err)
	defer func() { _ = res.Body.Close() }()
	body, _ := io.ReadAll(res.Body)
	assert.Equal(t, http.StatusOK, res.StatusCode)
	assert.Equal(t, "PK", string(body))
}

func TestConfirmBeforeUploadIsRejectedAndUnauthenticatedCallsAreRefused(t *testing.T) {
	s := startStack(t)

	status, ticket := s.call(t, http.MethodPost, "/v1/videos", `{"filename":"ferias.mp4","contentType":"video/mp4","sizeBytes":1000}`)
	require.Equal(t, http.StatusCreated, status)
	status, body := s.call(t, http.MethodPost, "/v1/videos/"+ticket["videoId"].(string)+"/process", "")
	assert.Equal(t, http.StatusConflict, status)
	assert.Equal(t, "upload_not_found", body["error"])

	res, err := http.Get(s.server.URL + "/v1/videos")
	require.NoError(t, err)
	_ = res.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, res.StatusCode)
}

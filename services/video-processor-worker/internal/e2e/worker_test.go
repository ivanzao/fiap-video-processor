//go:build integration

package e2e_test

import (
	"context"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ivanzao/fiap-video-processor/services/video-processor-worker/internal/adapters/inbound/sqs"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-worker/internal/adapters/outbound/ffmpeg"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-worker/internal/adapters/outbound/mail"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-worker/internal/adapters/outbound/postgres"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-worker/internal/adapters/outbound/s3"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-worker/internal/adapters/outbound/sns"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-worker/internal/adapters/outbound/workspace"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-worker/internal/adapters/outbound/zip"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-worker/internal/domain/video"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-worker/internal/platform/events"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-worker/internal/platform/system"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-worker/internal/testinfra"
)

const bucket = "video-processor"

type stack struct {
	aws         testinfra.AWS
	mailpit     testinfra.Mailpit
	repo        *postgres.ExecutionRepository
	consumer    *sqs.Consumer
	apiTopic    string
	apiInbox    string
	workerInbox string
}

func startStack(t *testing.T) stack {
	t.Helper()
	bin, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed")
	}
	pool := testinfra.StartPostgres(t)
	cloud := testinfra.StartLocalStack(t)
	mailpit := testinfra.StartMailpit(t)
	cloud.CreateBucket(t, bucket)
	apiTopic := cloud.CreateTopic(t, "video-processor-api-events")
	workerTopic := cloud.CreateTopic(t, "video-processor-worker-events")
	workerInbox := cloud.CreateQueueSubscribedTo(t, "video-processor-worker-inbox", apiTopic)
	apiInbox := cloud.CreateQueueSubscribedTo(t, "video-processor-api-inbox", workerTopic)

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	repo := postgres.NewExecutionRepository(pool)
	svc := video.NewService(video.Dependencies{
		Store:     s3.NewStore(cloud.S3, bucket),
		Extractor: ffmpeg.NewExtractor(bin, 1),
		Packager:  zip.Packager{},
		Workspace: workspace.NewTemp(t.TempDir()),
		Repo:      repo,
		Publisher: sns.NewPublisher(cloud.SNS, workerTopic, system.UUIDGenerator{}, system.Clock{}),
		Notifier:  mail.NewSMTPNotifier(mailpit.SMTPAddr, "noreply@fiapx.test"),
		Clock:     system.Clock{},
		IDs:       system.UUIDGenerator{},
	}, video.Config{FrameRate: 1, MaxAttempts: 3})
	consumer := sqs.NewConsumer(cloud.SQS, svc, log, sqs.Config{
		QueueURL: workerInbox, Concurrency: 2, VisibilityTimeout: 60 * time.Second, Heartbeat: 20 * time.Second, RetryDelay: time.Second,
	})
	return stack{aws: cloud, mailpit: mailpit, repo: repo, consumer: consumer, apiTopic: apiTopic, apiInbox: apiInbox, workerInbox: workerInbox}
}

func syntheticVideo(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ferias.mp4")
	cmd := exec.Command("ffmpeg", "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "testsrc=size=64x64:rate=10", "-t", "3", "-pix_fmt", "yuv420p", "-y", path)
	require.NoError(t, cmd.Run())
	return path
}

func (s stack) upload(t *testing.T, key, path string) {
	t.Helper()
	f, err := os.Open(path)
	require.NoError(t, err)
	defer func() { _ = f.Close() }()
	_, err = s.aws.S3.PutObject(context.Background(), &awss3.PutObjectInput{Bucket: aws.String(bucket), Key: aws.String(key), Body: f})
	require.NoError(t, err)
}

func (s stack) requestProcessing(t *testing.T, requestID, objectKey string) {
	t.Helper()
	s.aws.PublishJSON(t, s.apiTopic, events.Wrap(uuid.NewString(), time.Now(), events.VideoProcessingRequested{
		RequestID: requestID, VideoID: "vid-" + requestID, UserID: "usr-ana", UserEmail: "ana@example.com",
		ObjectKey: objectKey, Filename: "ferias.mp4",
	}))
}

func (s stack) collectWorkerEvents(t *testing.T, want int) map[string][]events.Envelope {
	t.Helper()
	got := map[string][]events.Envelope{}
	total := 0
	deadline := time.Now().Add(30 * time.Second)
	for total < want && time.Now().Before(deadline) {
		for _, body := range s.aws.Receive(t, s.apiInbox) {
			env, err := events.Decode([]byte(body))
			require.NoError(t, err)
			got[env.EventType] = append(got[env.EventType], env)
			total++
		}
	}
	return got
}

func TestRequestedVideoIsProcessedIntoAZipWithEventsAndEmail(t *testing.T) {
	s := startStack(t)
	ctx, stop := context.WithCancel(t.Context())
	defer stop()
	go s.consumer.Run(ctx)
	requestID := uuid.NewString()
	s.upload(t, "uploads/usr-ana/vid-1.mp4", syntheticVideo(t))

	s.requestProcessing(t, requestID, "uploads/usr-ana/vid-1.mp4")

	got := s.collectWorkerEvents(t, 2)
	require.Len(t, got[events.TypeVideoProcessingStarted], 1)
	require.Len(t, got[events.TypeVideoProcessingCompleted], 1)
	completed, err := events.PayloadAs[events.VideoProcessingCompleted](got[events.TypeVideoProcessingCompleted][0])
	require.NoError(t, err)
	assert.Equal(t, requestID, completed.RequestID)
	assert.Equal(t, 3, completed.FrameCount)
	assert.Equal(t, "results/"+requestID+".zip", completed.ZipKey)

	head, err := s.aws.S3.HeadObject(ctx, &awss3.HeadObjectInput{Bucket: aws.String(bucket), Key: aws.String(completed.ZipKey)})
	require.NoError(t, err)
	assert.Equal(t, "application/zip", aws.ToString(head.ContentType))

	exec, err := s.repo.FindExecution(ctx, requestID)
	require.NoError(t, err)
	assert.Equal(t, video.OutcomeCompleted, exec.Outcome)
	assert.Equal(t, 3, exec.FrameCount)

	require.Eventually(t, func() bool { return len(s.mailpit.Messages(t)) == 1 }, 10*time.Second, 200*time.Millisecond)
	email := s.mailpit.Messages(t)[0]
	assert.Equal(t, []string{"ana@example.com"}, email.To)
	assert.Equal(t, "Seu vídeo ferias.mp4 foi processado", email.Subject)
	assert.Empty(t, s.aws.Receive(t, s.workerInbox), "message acknowledged after publishing")
}

func TestUnprocessableVideoFailsOnceWithEmailAndNoRetry(t *testing.T) {
	s := startStack(t)
	ctx, stop := context.WithCancel(t.Context())
	defer stop()
	go s.consumer.Run(ctx)
	requestID := uuid.NewString()
	garbage := filepath.Join(t.TempDir(), "garbage.mp4")
	require.NoError(t, os.WriteFile(garbage, []byte("definitely not a video"), 0o600))
	s.upload(t, "uploads/usr-ana/vid-2.mp4", garbage)

	s.requestProcessing(t, requestID, "uploads/usr-ana/vid-2.mp4")

	got := s.collectWorkerEvents(t, 2)
	require.Len(t, got[events.TypeVideoProcessingFailed], 1)
	failed, err := events.PayloadAs[events.VideoProcessingFailed](got[events.TypeVideoProcessingFailed][0])
	require.NoError(t, err)
	assert.False(t, failed.Retryable)
	assert.Equal(t, 1, failed.Attempt)
	assert.Contains(t, failed.Reason, "unprocessable video")
	exec, err := s.repo.FindExecution(ctx, requestID)
	require.NoError(t, err)
	assert.Equal(t, video.OutcomeFailed, exec.Outcome)
	require.Eventually(t, func() bool { return len(s.mailpit.Messages(t)) == 1 }, 10*time.Second, 200*time.Millisecond)
	assert.Equal(t, "Falha ao processar o vídeo ferias.mp4", s.mailpit.Messages(t)[0].Subject)
}

//go:build integration

package s3_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ivanzao/fiap-video-processor/services/video-processor-worker/internal/adapters/outbound/s3"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-worker/internal/domain/video"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-worker/internal/testinfra"
)

const bucket = "video-processor"

func writeFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "frames.zip")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func TestStoreAgainstLocalStack(t *testing.T) {
	cloud := testinfra.StartLocalStack(t)
	cloud.CreateBucket(t, bucket)
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	store, err := s3.Connect(t.Context(), s3.Config{Region: "us-east-1", Endpoint: cloud.Endpoint, Bucket: bucket})
	require.NoError(t, err)
	missingBucket := s3.NewStore(cloud.S3, "no-such-bucket")

	t.Run("upload then download round-trips the file", func(t *testing.T) {
		ctx := t.Context()
		key := "results/usr-ana/req-1.zip"

		require.NoError(t, store.Upload(ctx, writeFile(t, "zip-bytes"), key, "application/zip"))

		head, err := cloud.S3.HeadObject(ctx, &awss3.HeadObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
		require.NoError(t, err)
		assert.Equal(t, "application/zip", aws.ToString(head.ContentType))
		assert.Equal(t, int64(len("zip-bytes")), aws.ToInt64(head.ContentLength))
		dst := filepath.Join(t.TempDir(), "copy.zip")
		require.NoError(t, store.Download(ctx, key, dst))
		got, err := os.ReadFile(dst)
		require.NoError(t, err)
		assert.Equal(t, "zip-bytes", string(got))
	})

	t.Run("download reports missing objects", func(t *testing.T) {
		err := store.Download(t.Context(), "uploads/usr-ana/missing.mp4", filepath.Join(t.TempDir(), "video.mp4"))

		assert.ErrorIs(t, err, video.ErrObjectNotFound)
	})

	t.Run("download reports other storage failures", func(t *testing.T) {
		err := missingBucket.Download(t.Context(), "uploads/usr-ana/vid-1.mp4", filepath.Join(t.TempDir(), "video.mp4"))

		require.Error(t, err)
		assert.NotErrorIs(t, err, video.ErrObjectNotFound)
	})

	t.Run("download fails when the destination cannot be created", func(t *testing.T) {
		ctx := t.Context()
		key := "uploads/usr-ana/vid-1.mp4"
		_, err := cloud.S3.PutObject(ctx, &awss3.PutObjectInput{Bucket: aws.String(bucket), Key: aws.String(key), Body: strings.NewReader("video")})
		require.NoError(t, err)

		err = store.Download(ctx, key, filepath.Join(t.TempDir(), "missing", "video.mp4"))

		assert.ErrorContains(t, err, "s3: create")
	})

	t.Run("upload fails when the source is missing", func(t *testing.T) {
		err := store.Upload(t.Context(), filepath.Join(t.TempDir(), "missing.zip"), "results/usr-ana/req-1.zip", "application/zip")

		assert.ErrorContains(t, err, "s3: open")
	})

	t.Run("upload reports storage failures", func(t *testing.T) {
		err := missingBucket.Upload(t.Context(), writeFile(t, "zip-bytes"), "results/usr-ana/req-1.zip", "application/zip")

		assert.ErrorContains(t, err, "s3: put object")
	})
}

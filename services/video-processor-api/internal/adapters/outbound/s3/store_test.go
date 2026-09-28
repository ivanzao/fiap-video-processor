//go:build integration

package s3_test

import (
	"bytes"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ivanzao/fiap-video-processor/services/video-processor-api/internal/adapters/outbound/s3"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-api/internal/domain/video"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-api/internal/testinfra"
)

func put(t *testing.T, ticket video.PresignedUpload, content []byte) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPut, ticket.URL, bytes.NewReader(content))
	require.NoError(t, err)
	for name, value := range ticket.Headers {
		req.Header.Set(name, value)
	}
	req.ContentLength = int64(len(content))
	res, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	_ = res.Body.Close()
	require.Equal(t, http.StatusOK, res.StatusCode)
}

func TestStoreAgainstLocalStack(t *testing.T) {
	cloud := testinfra.StartLocalStack(t)
	cloud.CreateBucket(t, bucket)
	store := s3.NewStore(cloud.S3, cloud.S3, bucket)

	t.Run("presigned upload carries the owner that head reads back", func(t *testing.T) {
		ctx := t.Context()
		content := bytes.Repeat([]byte("frame"), 20)
		key := "uploads/usr-ana/vid-1.mp4"
		ticket, err := store.PresignUpload(ctx, key, "video/mp4", int64(len(content)), owner, 5*time.Minute)
		require.NoError(t, err)

		put(t, ticket, content)

		info, err := store.Head(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, owner, info.Owner)
	})

	t.Run("head reports missing uploads", func(t *testing.T) {
		_, err := store.Head(t.Context(), "uploads/usr-ana/missing.mp4")

		assert.ErrorIs(t, err, video.ErrUploadNotFound)
	})

	t.Run("presigned download serves the object", func(t *testing.T) {
		ctx := t.Context()
		content := []byte("zip-bytes")
		key := "results/usr-ana/req-1.zip"
		ticket, err := store.PresignUpload(ctx, key, "application/zip", int64(len(content)), owner, 5*time.Minute)
		require.NoError(t, err)
		put(t, ticket, content)

		url, err := store.PresignDownload(ctx, key, 5*time.Minute)

		require.NoError(t, err)
		res, err := http.Get(url)
		require.NoError(t, err)
		defer func() { _ = res.Body.Close() }()
		require.Equal(t, http.StatusOK, res.StatusCode)
		body, err := io.ReadAll(res.Body)
		require.NoError(t, err)
		assert.Equal(t, content, body)
	})
}

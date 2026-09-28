package s3_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ivanzao/fiap-video-processor/services/video-processor-api/internal/adapters/outbound/s3"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-api/internal/domain/video"
)

const bucket = "video-processor"

var owner = video.ObjectOwner{UserID: "usr-ana", VideoID: "vid-1"}

func connect(t *testing.T, endpoint, publicEndpoint string) *s3.Store {
	t.Helper()
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	store, err := s3.Connect(t.Context(), s3.Config{Region: "us-east-1", Endpoint: endpoint, PublicEndpoint: publicEndpoint, Bucket: bucket})
	require.NoError(t, err)
	return store
}

func TestPresignUploadSignsTheOwnerIntoTheURLAndHeaders(t *testing.T) {
	store := connect(t, "http://localstack:4566", "")

	ticket, err := store.PresignUpload(t.Context(), "uploads/usr-ana/vid-1.mp4", "video/mp4", 1000, owner, 15*time.Minute)

	require.NoError(t, err)
	u, err := url.Parse(ticket.URL)
	require.NoError(t, err)
	assert.Equal(t, "localstack:4566", u.Host)
	assert.Equal(t, "/"+bucket+"/uploads/usr-ana/vid-1.mp4", u.Path)
	assert.Equal(t, "900", u.Query().Get("X-Amz-Expires"))
	for _, signed := range []string{"content-type", "x-amz-meta-user-id", "x-amz-meta-video-id"} {
		assert.Contains(t, u.Query().Get("X-Amz-SignedHeaders"), signed)
	}
	assert.Equal(t, map[string]string{
		"Content-Type":        "video/mp4",
		"x-amz-meta-user-id":  "usr-ana",
		"x-amz-meta-video-id": "vid-1",
	}, ticket.Headers)
}

func TestPresignedURLsUseThePublicEndpointWhenSet(t *testing.T) {
	store := connect(t, "http://localstack:4566", "http://localhost:4566")

	upload, err := store.PresignUpload(t.Context(), "uploads/usr-ana/vid-1.mp4", "video/mp4", 1000, owner, time.Minute)
	require.NoError(t, err)
	download, err := store.PresignDownload(t.Context(), "results/usr-ana/req-1.zip", time.Hour)
	require.NoError(t, err)

	for _, raw := range []string{upload.URL, download} {
		u, err := url.Parse(raw)
		require.NoError(t, err)
		assert.Equal(t, "localhost:4566", u.Host)
	}
	u, _ := url.Parse(download)
	assert.Equal(t, "/"+bucket+"/results/usr-ana/req-1.zip", u.Path)
	assert.Equal(t, "3600", u.Query().Get("X-Amz-Expires"))
}

func TestHeadMapsStorageAnswers(t *testing.T) {
	for name, tc := range map[string]struct {
		status   int
		metadata map[string]string
		want     video.ObjectInfo
		wantErr  error
		wrapped  bool
	}{
		"owner metadata": {
			status:   http.StatusOK,
			metadata: map[string]string{"X-Amz-Meta-User-Id": "usr-ana", "X-Amz-Meta-Video-Id": "vid-1"},
			want:     video.ObjectInfo{Owner: owner},
		},
		"missing object": {status: http.StatusNotFound, wantErr: video.ErrUploadNotFound},
		"denied":         {status: http.StatusForbidden, wrapped: true},
	} {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodHead, r.Method)
				assert.Equal(t, "/"+bucket+"/uploads/usr-ana/vid-1.mp4", r.URL.Path)
				for k, v := range tc.metadata {
					w.Header().Set(k, v)
				}
				w.WriteHeader(tc.status)
			}))
			t.Cleanup(server.Close)
			store := connect(t, server.URL, "")

			info, err := store.Head(t.Context(), "uploads/usr-ana/vid-1.mp4")

			switch {
			case tc.wantErr != nil:
				assert.ErrorIs(t, err, tc.wantErr)
			case tc.wrapped:
				require.Error(t, err)
				assert.NotErrorIs(t, err, video.ErrUploadNotFound)
				assert.Contains(t, err.Error(), "s3: head object")
			default:
				require.NoError(t, err)
				assert.Equal(t, tc.want, info)
			}
		})
	}
}

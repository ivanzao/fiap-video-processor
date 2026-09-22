package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ivanzao/fiap-video-processor/services/video-processor-api/internal/adapters/inbound/httpapi"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-api/internal/domain/video"
)

var now = time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)

type stubService struct {
	uploadErr, processErr, downloadErr error
	seen                               video.Identity
	items                              []video.RequestSummary
}

func (s *stubService) CreateVideo(_ context.Context, identity video.Identity, cmd video.UploadCommand) (video.UploadTicket, error) {
	s.seen = identity
	if s.uploadErr != nil {
		return video.UploadTicket{}, s.uploadErr
	}
	return video.UploadTicket{VideoID: "vid-1", UploadURL: "https://store/" + cmd.Filename, UploadHeaders: map[string]string{"x-amz-meta-user-id": identity.UserID}, ExpiresAt: now}, nil
}

func (s *stubService) ProcessVideo(_ context.Context, identity video.Identity, videoID string) (video.RequestSummary, error) {
	s.seen = identity
	if s.processErr != nil {
		return video.RequestSummary{}, s.processErr
	}
	return video.RequestSummary{
		Request: video.ProcessRequest{ID: "req-1", VideoID: videoID, UserID: identity.UserID, Status: video.StatusPending, CreatedAt: now, UpdatedAt: now},
		Video:   video.Video{ID: videoID, Filename: "ferias.mp4"},
	}, nil
}

func (s *stubService) ListVideos(_ context.Context, identity video.Identity) ([]video.RequestSummary, error) {
	s.seen = identity
	return s.items, nil
}

func (s *stubService) DownloadResult(_ context.Context, identity video.Identity, videoID string) (string, error) {
	s.seen = identity
	if s.downloadErr != nil {
		return "", s.downloadErr
	}
	return "https://store/results/" + videoID + ".zip", nil
}

func do(t *testing.T, h http.Handler, method, target, body string, authenticated bool) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if authenticated {
		req.Header.Set("X-User-Id", "usr-ana")
		req.Header.Set("X-User-Email", "ana@example.com")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decode(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	return out
}

func TestHealthIsPublic(t *testing.T) {
	rec := do(t, httpapi.NewRouter(&stubService{}, nil), http.MethodGet, "/health", "", false)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestProtectedRoutesRequireIdentity(t *testing.T) {
	h := httpapi.NewRouter(&stubService{}, nil)
	for _, tc := range []struct{ method, target string }{
		{http.MethodPost, "/v1/videos"}, {http.MethodPost, "/v1/videos/vid-1/process"},
		{http.MethodGet, "/v1/videos"}, {http.MethodGet, "/v1/videos/vid-1/download"},
	} {
		rec := do(t, h, tc.method, tc.target, "{}", false)
		assert.Equal(t, http.StatusUnauthorized, rec.Code, tc.target)
		assert.Equal(t, "unauthenticated", decode(t, rec)["error"], tc.target)
	}
}

func TestCreateVideoReturnsTicketWithCreated(t *testing.T) {
	svc := &stubService{}
	rec := do(t, httpapi.NewRouter(svc, nil), http.MethodPost, "/v1/videos", `{"filename":"ferias.mp4","contentType":"video/mp4","sizeBytes":1000}`, true)

	assert.Equal(t, http.StatusCreated, rec.Code)
	body := decode(t, rec)
	assert.Equal(t, "vid-1", body["videoId"])
	assert.Equal(t, "https://store/ferias.mp4", body["uploadUrl"])
	assert.Equal(t, map[string]any{"x-amz-meta-user-id": "usr-ana"}, body["uploadHeaders"])
	assert.Equal(t, video.Identity{UserID: "usr-ana", Email: "ana@example.com"}, svc.seen)
}

func TestCreateVideoMapsDomainErrorsToStatusCodes(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
		code   string
	}{
		{video.ErrUnsupportedFormat, http.StatusUnprocessableEntity, "unsupported_format"},
		{video.ErrVideoTooLarge, http.StatusUnprocessableEntity, "video_too_large"},
	} {
		rec := do(t, httpapi.NewRouter(&stubService{uploadErr: tc.err}, nil), http.MethodPost, "/v1/videos", `{"filename":"x","contentType":"y","sizeBytes":1}`, true)
		assert.Equal(t, tc.status, rec.Code)
		assert.Equal(t, tc.code, decode(t, rec)["error"])
	}
}

func TestCreateVideoRejectsMalformedJSON(t *testing.T) {
	rec := do(t, httpapi.NewRouter(&stubService{}, nil), http.MethodPost, "/v1/videos", `{"filename":`, true)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestProcessVideoReturnsAcceptedWithPendingRequest(t *testing.T) {
	rec := do(t, httpapi.NewRouter(&stubService{}, nil), http.MethodPost, "/v1/videos/vid-9/process", "", true)

	assert.Equal(t, http.StatusAccepted, rec.Code)
	body := decode(t, rec)
	assert.Equal(t, "req-1", body["requestId"])
	assert.Equal(t, "vid-9", body["videoId"])
	assert.Equal(t, "ferias.mp4", body["filename"])
	assert.Equal(t, "PENDING", body["status"])
}

func TestProcessVideoMapsErrors(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
	}{
		{video.ErrNotFound, http.StatusNotFound},
		{video.ErrForbidden, http.StatusForbidden},
		{video.ErrUploadNotFound, http.StatusConflict},
	} {
		rec := do(t, httpapi.NewRouter(&stubService{processErr: tc.err}, nil), http.MethodPost, "/v1/videos/vid-9/process", "", true)
		assert.Equal(t, tc.status, rec.Code)
	}
}

func TestListReturnsRequestsWithVideoAndResult(t *testing.T) {
	svc := &stubService{items: []video.RequestSummary{{
		Request: video.ProcessRequest{ID: "req-1", VideoID: "vid-1", Status: video.StatusCompleted, Attempts: 1, CreatedAt: now, UpdatedAt: now},
		Video:   video.Video{ID: "vid-1", Filename: "ferias.mp4"},
		Result:  &video.ProcessResult{ZipKey: "results/req-1.zip", FrameCount: 42},
	}}}
	rec := do(t, httpapi.NewRouter(svc, nil), http.MethodGet, "/v1/videos", "", true)

	assert.Equal(t, http.StatusOK, rec.Code)
	items := decode(t, rec)["items"].([]any)
	require.Len(t, items, 1)
	item := items[0].(map[string]any)
	assert.Equal(t, "ferias.mp4", item["filename"])
	assert.Equal(t, "COMPLETED", item["status"])
	assert.Equal(t, float64(42), item["result"].(map[string]any)["frameCount"])
}

func TestListOmitsFrameCountForFailedResults(t *testing.T) {
	svc := &stubService{items: []video.RequestSummary{{
		Request: video.ProcessRequest{ID: "req-1", VideoID: "vid-1", Status: video.StatusFailed, Attempts: 1, CreatedAt: now, UpdatedAt: now},
		Video:   video.Video{ID: "vid-1", Filename: "ferias.mp4"},
		Result:  &video.ProcessResult{FailureReason: "unprocessable video"},
	}}}
	rec := do(t, httpapi.NewRouter(svc, nil), http.MethodGet, "/v1/videos", "", true)

	result := decode(t, rec)["items"].([]any)[0].(map[string]any)["result"].(map[string]any)
	assert.Equal(t, "unprocessable video", result["failureReason"])
	_, hasFrames := result["frameCount"]
	assert.False(t, hasFrames)
}

func TestListReturnsEmptyArrayNotNull(t *testing.T) {
	rec := do(t, httpapi.NewRouter(&stubService{}, nil), http.MethodGet, "/v1/videos", "", true)

	assert.JSONEq(t, `{"items":[]}`, rec.Body.String())
}

func TestDownloadReturnsPresignedURLAndMapsNotCompleted(t *testing.T) {
	rec := do(t, httpapi.NewRouter(&stubService{}, nil), http.MethodGet, "/v1/videos/vid-1/download", "", true)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "https://store/results/vid-1.zip", decode(t, rec)["downloadUrl"])

	rec = do(t, httpapi.NewRouter(&stubService{downloadErr: video.ErrNotCompleted}, nil), http.MethodGet, "/v1/videos/vid-1/download", "", true)
	assert.Equal(t, http.StatusConflict, rec.Code)
}

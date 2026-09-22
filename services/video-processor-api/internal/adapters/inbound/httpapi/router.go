package httpapi

import (
	"context"
	"io/fs"
	"net/http"

	"github.com/ivanzao/fiap-video-processor/services/video-processor-api/internal/domain/video"
)

type VideoService interface {
	CreateVideo(ctx context.Context, identity video.Identity, cmd video.UploadCommand) (video.UploadTicket, error)
	ProcessVideo(ctx context.Context, identity video.Identity, videoID string) (video.RequestSummary, error)
	ListVideos(ctx context.Context, identity video.Identity) ([]video.RequestSummary, error)
	DownloadResult(ctx context.Context, identity video.Identity, videoID string) (string, error)
}

func NewRouter(svc VideoService, static fs.FS) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	h := &videoHandler{svc: svc}
	mux.Handle("POST /v1/videos", RequireIdentity(http.HandlerFunc(h.createVideo)))
	mux.Handle("POST /v1/videos/{id}/process", RequireIdentity(http.HandlerFunc(h.processVideo)))
	mux.Handle("GET /v1/videos", RequireIdentity(http.HandlerFunc(h.listVideos)))
	mux.Handle("GET /v1/videos/{id}/download", RequireIdentity(http.HandlerFunc(h.downloadResult)))
	if static != nil {
		mux.Handle("GET /", http.FileServerFS(static))
	}
	return mux
}

package httpapi

import (
	"time"

	"github.com/ivanzao/fiap-video-processor/services/video-processor-api/internal/domain/video"
)

type uploadRequest struct {
	Filename    string `json:"filename"`
	ContentType string `json:"contentType"`
	SizeBytes   int64  `json:"sizeBytes"`
}

type uploadResponse struct {
	VideoID       string            `json:"videoId"`
	UploadURL     string            `json:"uploadUrl"`
	UploadHeaders map[string]string `json:"uploadHeaders"`
	ExpiresAt     time.Time         `json:"expiresAt"`
}

type resultResponse struct {
	ZipKey        string `json:"zipKey,omitempty"`
	FrameCount    *int   `json:"frameCount,omitempty"`
	FailureReason string `json:"failureReason,omitempty"`
}

type requestResponse struct {
	RequestID string          `json:"requestId"`
	VideoID   string          `json:"videoId"`
	Filename  string          `json:"filename,omitempty"`
	Status    video.Status    `json:"status"`
	Attempts  int             `json:"attempts"`
	CreatedAt time.Time       `json:"createdAt"`
	UpdatedAt time.Time       `json:"updatedAt"`
	Result    *resultResponse `json:"result,omitempty"`
}

type listResponse struct {
	Items []requestResponse `json:"items"`
}

type downloadResponse struct {
	DownloadURL string `json:"downloadUrl"`
}

func toUploadResponse(t video.UploadTicket) uploadResponse {
	return uploadResponse{VideoID: t.VideoID, UploadURL: t.UploadURL, UploadHeaders: t.UploadHeaders, ExpiresAt: t.ExpiresAt}
}

func toRequestResponse(s video.RequestSummary) requestResponse {
	req := s.Request
	out := requestResponse{
		RequestID: req.ID, VideoID: req.VideoID, Filename: s.Video.Filename, Status: req.Status,
		Attempts: req.Attempts, CreatedAt: req.CreatedAt, UpdatedAt: req.UpdatedAt,
	}
	if s.Result != nil {
		out.Result = &resultResponse{ZipKey: s.Result.ZipKey, FailureReason: s.Result.FailureReason}
		if s.Result.ZipKey != "" {
			frames := s.Result.FrameCount
			out.Result.FrameCount = &frames
		}
	}
	return out
}

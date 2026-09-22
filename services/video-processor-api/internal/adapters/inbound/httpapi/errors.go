package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/ivanzao/fiap-video-processor/services/video-processor-api/internal/domain/video"
)

type errorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorResponse{Error: code, Message: message})
}

func writeDomainError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, video.ErrUnsupportedFormat):
		writeError(w, http.StatusUnprocessableEntity, "unsupported_format", "video format is not supported")
	case errors.Is(err, video.ErrVideoTooLarge):
		writeError(w, http.StatusUnprocessableEntity, "video_too_large", "video exceeds the size limit")
	case errors.Is(err, video.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "video not found")
	case errors.Is(err, video.ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden", "video belongs to another user")
	case errors.Is(err, video.ErrUploadNotFound):
		writeError(w, http.StatusConflict, "upload_not_found", "upload has not reached storage yet")
	case errors.Is(err, video.ErrUploadMismatch):
		writeError(w, http.StatusConflict, "upload_mismatch", "uploaded object does not carry the owner of this video")
	case errors.Is(err, video.ErrNotCompleted):
		writeError(w, http.StatusConflict, "not_completed", "processing has not completed")
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", "unexpected error")
	}
}

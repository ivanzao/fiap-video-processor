package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/ivanzao/fiap-video-processor/services/video-processor-api/internal/domain/video"
)

type videoHandler struct {
	svc VideoService
}

func (h *videoHandler) createVideo(w http.ResponseWriter, r *http.Request) {
	identity := identityOf(r)
	var body uploadRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "request body must be valid JSON")
		return
	}
	ticket, err := h.svc.CreateVideo(r.Context(), identity, video.UploadCommand{
		Filename: body.Filename, ContentType: body.ContentType, SizeBytes: body.SizeBytes,
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toUploadResponse(ticket))
}

func (h *videoHandler) processVideo(w http.ResponseWriter, r *http.Request) {
	summary, err := h.svc.ProcessVideo(r.Context(), identityOf(r), r.PathValue("id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, toRequestResponse(summary))
}

func (h *videoHandler) listVideos(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.ListVideos(r.Context(), identityOf(r))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	out := listResponse{Items: make([]requestResponse, 0, len(items))}
	for _, it := range items {
		out.Items = append(out.Items, toRequestResponse(it))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *videoHandler) downloadResult(w http.ResponseWriter, r *http.Request) {
	url, err := h.svc.DownloadResult(r.Context(), identityOf(r), r.PathValue("id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, downloadResponse{DownloadURL: url})
}

func identityOf(r *http.Request) video.Identity {
	id, _ := IdentityFromContext(r.Context())
	return video.Identity{UserID: id.UserID, Email: id.Email}
}

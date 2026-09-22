package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/ivanzao/fiap-video-processor/services/video-processor-auth/internal/domain/user"
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
	case errors.Is(err, user.ErrInvalidEmail):
		writeError(w, http.StatusBadRequest, "invalid_email", "email is not valid")
	case errors.Is(err, user.ErrWeakPassword):
		writeError(w, http.StatusBadRequest, "weak_password", "password must have at least 8 characters")
	case errors.Is(err, user.ErrEmailTaken):
		writeError(w, http.StatusConflict, "email_taken", "email is already registered")
	case errors.Is(err, user.ErrInvalidCredentials):
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "email or password is incorrect")
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", "unexpected failure")
	}
}

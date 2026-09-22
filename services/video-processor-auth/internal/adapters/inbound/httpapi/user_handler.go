package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/ivanzao/fiap-video-processor/services/video-processor-auth/internal/domain/user"
)

func SignUpHandler(svc UserService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		creds, ok := decodeCredentials(w, r)
		if !ok {
			return
		}
		u, err := svc.SignUp(r.Context(), creds)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, signUpResponse{ID: u.ID, Email: u.Email})
	})
}

func LoginHandler(svc UserService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		creds, ok := decodeCredentials(w, r)
		if !ok {
			return
		}
		session, err := svc.Login(r.Context(), creds)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, loginResponse{Token: session.Token})
	})
}

func decodeCredentials(w http.ResponseWriter, r *http.Request) (user.Credentials, bool) {
	var req credentialsRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "body must be a JSON object with email and password")
		return user.Credentials{}, false
	}
	return user.Credentials{Email: req.Email, Password: req.Password}, true
}

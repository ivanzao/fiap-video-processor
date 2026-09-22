package httpapi_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ivanzao/fiap-video-processor/services/video-processor-auth/internal/adapters/inbound/httpapi"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-auth/internal/domain/user"
)

type fakeService struct {
	signUpErr error
	loginErr  error
	gotCreds  user.Credentials
}

func (f *fakeService) SignUp(_ context.Context, creds user.Credentials) (user.User, error) {
	f.gotCreds = creds
	return user.User{ID: "usr-1", Email: creds.Email}, f.signUpErr
}

func (f *fakeService) Login(_ context.Context, creds user.Credentials) (user.Session, error) {
	f.gotCreds = creds
	return user.Session{Token: "jwt-123"}, f.loginErr
}

func post(router http.Handler, path, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, strings.NewReader(body)))
	return rec
}

func TestSignUpReturnsCreatedUser(t *testing.T) {
	svc := &fakeService{}

	rec := post(httpapi.NewRouter(svc), "/auth/v1/signup", `{"email":"ana@example.com","password":"correct horse"}`)

	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.JSONEq(t, `{"id":"usr-1","email":"ana@example.com"}`, rec.Body.String())
	assert.Equal(t, user.Credentials{Email: "ana@example.com", Password: "correct horse"}, svc.gotCreds)
}

func TestLoginReturnsToken(t *testing.T) {
	rec := post(httpapi.NewRouter(&fakeService{}), "/auth/v1/login", `{"email":"ana@example.com","password":"correct horse"}`)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"token":"jwt-123"}`, rec.Body.String())
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
}

func TestMalformedBodyIsRejected(t *testing.T) {
	rec := post(httpapi.NewRouter(&fakeService{}), "/auth/v1/login", `{"email":`)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), `"error":"invalid_body"`)
}

func TestDomainErrorsMapToStatusCodes(t *testing.T) {
	cases := map[string]struct {
		err    error
		status int
		code   string
	}{
		"invalid email":       {user.ErrInvalidEmail, http.StatusBadRequest, "invalid_email"},
		"weak password":       {user.ErrWeakPassword, http.StatusBadRequest, "weak_password"},
		"email taken":         {user.ErrEmailTaken, http.StatusConflict, "email_taken"},
		"invalid credentials": {user.ErrInvalidCredentials, http.StatusUnauthorized, "invalid_credentials"},
		"unexpected":          {errors.New("boom"), http.StatusInternalServerError, "internal_error"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			rec := post(httpapi.NewRouter(&fakeService{signUpErr: tc.err, loginErr: tc.err}), "/auth/v1/signup", `{"email":"a@b.co","password":"x"}`)
			assert.Equal(t, tc.status, rec.Code)
			assert.Contains(t, rec.Body.String(), `"error":"`+tc.code+`"`)
		})
	}
}

func TestHealthAndUnknownRoutes(t *testing.T) {
	router := httpapi.NewRouter(&fakeService{})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	assert.Equal(t, http.StatusOK, rec.Code)

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/auth/v1/login", nil))
	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

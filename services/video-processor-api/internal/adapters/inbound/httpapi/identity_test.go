package httpapi_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ivanzao/fiap-video-processor/services/video-processor-api/internal/adapters/inbound/httpapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIdentityFromRequestReadsGatewayHeaders(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/videos", nil)
	req.Header.Set("X-User-Id", "usr-1")
	req.Header.Set("X-User-Email", "ana@example.com")

	id, err := httpapi.IdentityFromRequest(req)

	require.NoError(t, err)
	assert.Equal(t, httpapi.Identity{UserID: "usr-1", Email: "ana@example.com"}, id)
}

func TestIdentityFromRequestFailsWhenUserIDMissing(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/videos", nil)
	req.Header.Set("X-User-Email", "ana@example.com")

	_, err := httpapi.IdentityFromRequest(req)

	assert.ErrorIs(t, err, httpapi.ErrUnauthenticated)
}

func TestRequireRejectsUnauthenticatedRequestsWith401(t *testing.T) {
	handler := httpapi.RequireIdentity(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/videos", nil))

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.JSONEq(t, `{"error":"unauthenticated","message":"request carries no authenticated identity"}`, rec.Body.String())
}

func TestRequireExposesIdentityThroughContext(t *testing.T) {
	var seen httpapi.Identity
	handler := httpapi.RequireIdentity(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen, _ = httpapi.IdentityFromContext(r.Context())
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodGet, "/v1/videos", nil)
	req.Header.Set("X-User-Id", "usr-7")
	req.Header.Set("X-User-Email", "bia@example.com")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, httpapi.Identity{UserID: "usr-7", Email: "bia@example.com"}, seen)
}

func TestIdentityFromContextReportsAbsence(t *testing.T) {
	_, ok := httpapi.IdentityFromContext(t.Context())

	assert.False(t, ok)
}

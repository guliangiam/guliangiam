package i18n

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	platformi18n "github.com/guliangiam/guliangiam/internal/platform/i18n"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// UpdateTranslation decodes r.Body directly, so an oversized body used to
// surface as invalid_argument 400. It must now be the registered 413.
func TestUpdateTranslation_OverBodyLimitIs413(t *testing.T) {
	router := chi.NewRouter()
	router.Put("/api/v1/i18n/translations/{language}",
		NewHandler(platformi18n.NewTranslator()).UpdateTranslation)

	payload := `{"key":"` + strings.Repeat("x", 2048) + `"}`
	request := httptest.NewRequest(http.MethodPut, "/api/v1/i18n/translations/en",
		strings.NewReader(payload))
	recorder := httptest.NewRecorder()
	request.Body = http.MaxBytesReader(recorder, request.Body, 1024)

	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusRequestEntityTooLarge, recorder.Code, recorder.Body.String())
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	assert.Equal(t, "request_entity_too_large", body.Error.Code)
}

// The success path must not change: a normal translation update still answers
// 200.
func TestUpdateTranslation_WithinLimitStillSucceeds(t *testing.T) {
	router := chi.NewRouter()
	router.Put("/api/v1/i18n/translations/{language}",
		NewHandler(platformi18n.NewTranslator()).UpdateTranslation)

	request := httptest.NewRequest(http.MethodPut, "/api/v1/i18n/translations/en",
		strings.NewReader(`{"auth.login":"Sign in"}`))
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
}

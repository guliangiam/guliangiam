package i18n

import (
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/stretchr/testify/assert"
)

func TestRegisterChiRoutes(t *testing.T) {
	r := chi.NewRouter()
	r.Use(chimw.Recoverer)
	h := &Handler{}
	RegisterChiRoutes(r, h)

	tests := []struct {
		method string
		path   string
	}{
		{"GET", "/api/v1/i18n/languages"},
		{"GET", "/api/v1/i18n/translations/en"},
		{"PUT", "/api/v1/i18n/translations/en"},
		{"GET", "/api/v1/i18n/translate"},
	}

	for _, tt := range tests {
		t.Run(tt.method+"_"+tt.path, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			assert.NotEqual(t, 0, w.Code)
		})
	}
}

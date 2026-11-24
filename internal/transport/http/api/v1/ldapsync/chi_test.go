package ldapsync

import (
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	appldapsync "github.com/guliangiam/guliangiam/internal/app/ldapsync"
	"github.com/stretchr/testify/assert"
)

func TestRegisterRoutes(t *testing.T) {
	r := chi.NewRouter()
	r.Use(chimw.Recoverer)
	h := &Handler{Service: &appldapsync.Service{}}
	RegisterRoutes(r, h)

	tests := []struct {
		method string
		path   string
	}{
		{"GET", "/api/v1/ldap-sync-configs/"},
		{"GET", "/api/v1/ldap-sync-configs/test-id"},
		{"DELETE", "/api/v1/ldap-sync-configs/test-id"},
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

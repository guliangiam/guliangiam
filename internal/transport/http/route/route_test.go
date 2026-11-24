package route

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func TestWriteJSON_NilValue(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteJSON(rec, http.StatusOK, nil)
	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if ct != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q, want application/json; charset=utf-8", ct)
	}
}

func TestWriteJSON_WithBody(t *testing.T) {
	rec := httptest.NewRecorder()
	data := map[string]string{"key": "value"}
	WriteJSON(rec, http.StatusOK, data)
	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"key"`) {
		t.Error("response body should contain key")
	}
}

func TestWriteJSON_CreatedStatus(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteJSON(rec, http.StatusCreated, map[string]string{"id": "123"})
	if rec.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, rec.Code)
	}
}

func TestStatusCodeFromError_Nil(t *testing.T) {
	code := StatusCodeFromError(nil)
	if code != http.StatusOK {
		t.Errorf("expected %d for nil error, got %d", http.StatusOK, code)
	}
}

func TestStatusCodeFromError_AppErrors(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode int
	}{
		{"invalid argument", apperrors.NewInvalidArgument("bad"), http.StatusBadRequest},
		{"unauthorized", apperrors.NewUnauthorized("no auth"), http.StatusUnauthorized},
		{"forbidden", apperrors.NewForbidden("denied"), http.StatusForbidden},
		{"not found", apperrors.NewNotFound("missing"), http.StatusNotFound},
		{"conflict", apperrors.NewConflict("dup"), http.StatusConflict},
		{"already exists", apperrors.NewAlreadyExists("exists"), http.StatusConflict},
		{"internal", apperrors.NewInternal("oops"), http.StatusInternalServerError},
		{"unimplemented", apperrors.NewUnimplemented("todo"), http.StatusNotImplemented},
		{"unavailable", apperrors.NewUnavailable("down"), http.StatusServiceUnavailable},
		{"too many requests", apperrors.NewTooManyRequests("slow"), http.StatusTooManyRequests},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := StatusCodeFromError(tt.err)
			if code != tt.wantCode {
				t.Errorf("StatusCodeFromError() = %d, want %d", code, tt.wantCode)
			}
		})
	}
}

func TestStatusCodeFromError_UnknownAppError(t *testing.T) {
	code := StatusCodeFromError(apperrors.New(apperrors.Code("unknown"), "msg"))
	if code != http.StatusInternalServerError {
		t.Errorf("expected %d for unknown code, got %d", http.StatusInternalServerError, code)
	}
}

func TestWriteError_Forbidden(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	WriteError(rec, req, apperrors.NewForbidden("access denied"))
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}
}

func TestDecodeJSON_ValidBody(t *testing.T) {
	body := strings.NewReader(`{"name":"test"}`)
	req := httptest.NewRequest(http.MethodPost, "/", body)
	req.Header.Set("Content-Type", "application/json")

	var result map[string]string
	err := DecodeJSON(req, &result)
	if err != nil {
		t.Errorf("DecodeJSON() error: %v", err)
	}
	if result["name"] != "test" {
		t.Errorf("name = %q, want %q", result["name"], "test")
	}
}

func TestDecodeJSON_InvalidJSON(t *testing.T) {
	body := strings.NewReader(`not-json`)
	req := httptest.NewRequest(http.MethodPost, "/", body)
	req.Header.Set("Content-Type", "application/json")

	var result map[string]string
	err := DecodeJSON(req, &result)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestDecodeJSON_UnknownFields(t *testing.T) {
	body := strings.NewReader(`{"name":"test","extra":"field"}`)
	req := httptest.NewRequest(http.MethodPost, "/", body)
	req.Header.Set("Content-Type", "application/json")

	var result struct {
		Name string `json:"name"`
	}
	err := DecodeJSON(req, &result)
	if err == nil {
		t.Error("expected error for unknown fields (DisallowUnknownFields)")
	}
}

func TestDefinition_Fields(t *testing.T) {
	d := Definition{
		Method:      http.MethodGet,
		Pattern:     "/api/test",
		OperationID: "getTest",
		Summary:     "Get test",
		Permission:  "test:read",
	}
	if d.Method != http.MethodGet {
		t.Errorf("Method = %q, want GET", d.Method)
	}
	if d.Permission != "test:read" {
		t.Errorf("Permission = %q, want test:read", d.Permission)
	}
}

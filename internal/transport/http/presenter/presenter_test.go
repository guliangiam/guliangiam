package presenter

import (
	"encoding/json"
	"errors"
	"testing"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	httperr "github.com/guliangiam/guliangiam/internal/platform/http"
)

func TestPresentError_Nil(t *testing.T) {
	result := PresentError(nil, "req-1", "trace-1")
	if result.Error.Code != "" {
		t.Errorf("expected empty code for nil error, got %q", result.Error.Code)
	}
}

func TestPresentError_AppError(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		wantCode  string
		wantMsg   string
		requestID string
		traceID   string
	}{
		{
			"forbidden",
			apperrors.NewForbidden("access denied"),
			"forbidden", "access denied", "req-1", "trace-1",
		},
		{
			"not found",
			apperrors.NewNotFound("resource missing"),
			"not_found", "resource missing", "", "",
		},
		{
			"with meta",
			apperrors.NewWithMeta(apperrors.CodeInvalidArgument, "bad input", map[string]any{"field": "email"}),
			"invalid_argument", "bad input", "req-2", "",
		},
		{
			"internal sanitized",
			apperrors.NewInternal("insert failed: SQLSTATE 23505 duplicate key"),
			"internal", internalErrorMessage, "req-3", "trace-3",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PresentError(tt.err, tt.requestID, tt.traceID)
			if result.Error.Code != tt.wantCode {
				t.Errorf("Code = %q, want %q", result.Error.Code, tt.wantCode)
			}
			if result.Error.Message != tt.wantMsg {
				t.Errorf("Message = %q, want %q", result.Error.Message, tt.wantMsg)
			}
			if result.RequestID != tt.requestID {
				t.Errorf("RequestID = %q, want %q", result.RequestID, tt.requestID)
			}
			if result.TraceID != tt.traceID {
				t.Errorf("TraceID = %q, want %q", result.TraceID, tt.traceID)
			}
		})
	}
}

func TestPresentError_MatchesPlatformCanonicalEnvelope(t *testing.T) {
	err := apperrors.NewWithMeta(
		apperrors.CodeUnprocessableEntity,
		"email must be an RFC 5322 address",
		map[string]any{"field": "email"},
	)
	presenterJSON, marshalErr := json.Marshal(PresentError(err, "req-e1b2", "trace-e1b2"))
	if marshalErr != nil {
		t.Fatalf("marshal presenter envelope: %v", marshalErr)
	}
	platformJSON, marshalErr := json.Marshal(httperr.PresentError(err, "req-e1b2", "trace-e1b2"))
	if marshalErr != nil {
		t.Fatalf("marshal platform envelope: %v", marshalErr)
	}
	if string(presenterJSON) != string(platformJSON) {
		t.Fatalf("presenter and platform envelopes differ:\n presenter: %s\n platform:  %s", presenterJSON, platformJSON)
	}
}

func TestPresentError_StandardError(t *testing.T) {
	result := PresentError(errors.New("some std error"), "req-1", "trace-1")
	if result.Error.Code != string(apperrors.CodeInternal) {
		t.Errorf("Code = %q, want %q", result.Error.Code, apperrors.CodeInternal)
	}
	if result.Error.Message != "an internal error occurred" {
		t.Errorf("Message = %q, want %q", result.Error.Message, "an internal error occurred")
	}
}

func TestPresentError_InternalDropsMessageKeyAndMeta(t *testing.T) {
	err := &apperrors.Error{
		Code:       apperrors.CodeInternal,
		Message:    "connection refused: postgres://user:secret@db/app",
		MessageKey: "internal.database",
		Meta:       map[string]any{"dsn": "postgres://user:secret@db/app"},
	}
	result := PresentError(err, "req-1", "trace-1")
	if result.Error.Message != internalErrorMessage {
		t.Errorf("Message = %q, want %q", result.Error.Message, internalErrorMessage)
	}
	if result.Error.MessageKey != "" {
		t.Errorf("MessageKey = %q, want empty for internal error", result.Error.MessageKey)
	}
	if result.Error.Meta != nil {
		t.Errorf("Meta = %v, want nil for internal error", result.Error.Meta)
	}
}

func TestNewListResponse(t *testing.T) {
	items := []string{"a", "b", "c"}
	result := NewListResponse(items, 1, 10, 100)

	if len(result.Items) != 3 {
		t.Errorf("Items length = %d, want 3", len(result.Items))
	}
	if result.Page.Page != 1 {
		t.Errorf("Page = %d, want 1", result.Page.Page)
	}
	if result.Page.PageSize != 10 {
		t.Errorf("PageSize = %d, want 10", result.Page.PageSize)
	}
	if result.Page.Total != 100 {
		t.Errorf("Total = %d, want 100", result.Page.Total)
	}
}

func TestNewListResponse_Empty(t *testing.T) {
	result := NewListResponse([]int{}, 1, 20, 0)
	if len(result.Items) != 0 {
		t.Errorf("Items length = %d, want 0", len(result.Items))
	}
}

// A nil result set must be indistinguishable on the wire from a non-nil empty
// one: `"items":null` breaks clients that iterate the array unconditionally.
func TestNewListResponse_NilItems(t *testing.T) {
	result := NewListResponse[int](nil, 1, 20, 0)
	if result.Items == nil {
		t.Fatal("Items = nil, want an empty slice")
	}

	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	want := `{"items":[],"page":{"page":1,"pageSize":20,"total":0}}`
	if string(encoded) != want {
		t.Errorf("encoded = %s, want %s", encoded, want)
	}
}

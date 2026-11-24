package entries

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5/middleware"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/protobuf/proto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type otlpErrorEnvelope struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	RequestID string `json:"requestId"`
	TraceID   string `json:"traceId"`
}

type otlpHandlerCase struct {
	name   string
	path   string
	handle func(Handler, http.ResponseWriter, *http.Request)
}

func otlpHandlerCases() []otlpHandlerCase {
	return []otlpHandlerCase{
		{name: "traces", path: "/api/v1/entries/traces", handle: Handler.AddOtlpTrace},
		{name: "metrics", path: "/api/v1/entries/metrics", handle: Handler.AddOtlpMetrics},
		{name: "logs", path: "/api/v1/entries/logs", handle: Handler.AddOtlpLogs},
	}
}

func serveOTLP(t *testing.T, handle func(Handler, http.ResponseWriter, *http.Request), req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	middleware.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handle(Handler{}, w, r)
	})).ServeHTTP(recorder, req)
	return recorder
}

func decodeOTLPEnvelope(t *testing.T, recorder *httptest.ResponseRecorder) otlpErrorEnvelope {
	t.Helper()
	var envelope otlpErrorEnvelope
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope), recorder.Body.String())
	return envelope
}

func TestOTLPErrorEnvelope_UnsupportedMediaType(t *testing.T) {
	for _, tc := range otlpHandlerCases() {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tc.path, nil)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Request-Id", "otlp-415-"+tc.name)

			recorder := serveOTLP(t, tc.handle, req)
			require.Equal(t, http.StatusUnsupportedMediaType, recorder.Code)
			assert.Equal(t, "application/json; charset=utf-8", recorder.Header().Get("Content-Type"))

			envelope := decodeOTLPEnvelope(t, recorder)
			assert.Equal(t, "unsupported_media_type", envelope.Error.Code)
			assert.Equal(t, "unsupported media type: expected application/x-protobuf", envelope.Error.Message)
			assert.Equal(t, "otlp-415-"+tc.name, envelope.RequestID)
			assert.Equal(t, envelope.RequestID, envelope.TraceID)
		})
	}
}

type failingReadCloser struct{}

func (failingReadCloser) Read([]byte) (int, error) { return 0, errors.New("read failed") }
func (failingReadCloser) Close() error             { return nil }

func TestOTLPErrorEnvelope_BodyReadFailure(t *testing.T) {
	for _, tc := range otlpHandlerCases() {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tc.path, nil)
			req.Header.Set("Content-Type", "application/x-protobuf")
			req.Header.Set("X-Request-Id", "otlp-400-"+tc.name)
			req.Body = failingReadCloser{}

			recorder := serveOTLP(t, tc.handle, req)
			require.Equal(t, http.StatusBadRequest, recorder.Code)

			envelope := decodeOTLPEnvelope(t, recorder)
			assert.Equal(t, "invalid_argument", envelope.Error.Code)
			assert.Equal(t, "failed to read request body", envelope.Error.Message)
			assert.Equal(t, "otlp-400-"+tc.name, envelope.RequestID)
			assert.Equal(t, envelope.RequestID, envelope.TraceID)
		})
	}
}

func TestOTLPErrorEnvelope_BodyTooLarge(t *testing.T) {
	for _, tc := range otlpHandlerCases() {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tc.path, bytes.NewReader(bytes.Repeat([]byte("x"), 64)))
			req.Header.Set("Content-Type", "application/x-protobuf")
			req.Header.Set("X-Request-Id", "otlp-413-"+tc.name)
			req.Body = http.MaxBytesReader(httptest.NewRecorder(), req.Body, 32)

			recorder := serveOTLP(t, tc.handle, req)
			require.Equal(t, http.StatusRequestEntityTooLarge, recorder.Code)

			envelope := decodeOTLPEnvelope(t, recorder)
			assert.Equal(t, "request_entity_too_large", envelope.Error.Code)
			assert.Equal(t, "request body exceeds the 32 byte limit", envelope.Error.Message)
			assert.Equal(t, "otlp-413-"+tc.name, envelope.RequestID)
			assert.Equal(t, envelope.RequestID, envelope.TraceID)
		})
	}
}

func TestWriteProtobufResponse_EncodeFailureUnifiedEnvelope(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/entries/traces", nil)
	req.Header.Set("X-Request-Id", "otlp-500-encode")
	recorder := httptest.NewRecorder()

	middleware.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeProtobufResponseWith(w, r, &coltracepb.ExportTraceServiceResponse{},
			func(proto.Message) ([]byte, error) { return nil, errors.New("encode failed") })
	})).ServeHTTP(recorder, req)

	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	envelope := decodeOTLPEnvelope(t, recorder)
	assert.Equal(t, "internal", envelope.Error.Code)
	assert.Equal(t, "an internal error occurred", envelope.Error.Message)
	assert.Equal(t, "otlp-500-encode", envelope.RequestID)
	assert.Equal(t, envelope.RequestID, envelope.TraceID)
}

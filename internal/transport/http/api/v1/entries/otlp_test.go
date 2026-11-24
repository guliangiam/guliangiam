package entries

import (
	"net/http"
	"net/http/httptest"
	"testing"

	collogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	colmetricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/protobuf/proto"
)

func TestIsProtobufContentType_CheckLogic(t *testing.T) {
	tests := []struct {
		name   string
		ct     string
		expect bool
	}{
		{"empty", "", false},
		{"json", "application/json", false},
		{"protobuf", "application/x-protobuf", true},
		{"with charset", "application/x-protobuf;charset=utf-8", true},
		{"case insensitive", "Application/X-Protobuf", true},
		{"other", "text/plain", false},
		{"malformed", "application/x-protobuf; charset", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/", nil)
			req.Header.Set("Content-Type", tt.ct)
			got := isProtobufContentType(req)
			if got != tt.expect {
				t.Errorf("isProtobufContentType(%q) = %v, want %v", tt.ct, got, tt.expect)
			}
		})
	}
}

func TestWriteProtobufResponse_TraceResponse(t *testing.T) {
	rec := httptest.NewRecorder()
	resp := &coltracepb.ExportTraceServiceResponse{}
	writeProtobufResponse(rec, httptest.NewRequest(http.MethodPost, "/", nil), resp)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/x-protobuf" {
		t.Errorf("Content-Type = %q, want application/x-protobuf", ct)
	}

	expected, _ := proto.Marshal(resp)
	if len(rec.Body.Bytes()) != len(expected) {
		t.Errorf("body length = %d, want %d", len(rec.Body.Bytes()), len(expected))
	}
}

func TestWriteProtobufResponse_MetricsResponse(t *testing.T) {
	rec := httptest.NewRecorder()
	resp := &colmetricspb.ExportMetricsServiceResponse{}
	writeProtobufResponse(rec, httptest.NewRequest(http.MethodPost, "/", nil), resp)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/x-protobuf" {
		t.Errorf("Content-Type = %q, want application/x-protobuf", ct)
	}
}

func TestWriteProtobufResponse_LogsResponse(t *testing.T) {
	rec := httptest.NewRecorder()
	resp := &collogspb.ExportLogsServiceResponse{}
	writeProtobufResponse(rec, httptest.NewRequest(http.MethodPost, "/", nil), resp)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/x-protobuf" {
		t.Errorf("Content-Type = %q, want application/x-protobuf", ct)
	}
}

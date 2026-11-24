package entries

import (
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"

	appentries "github.com/guliangiam/guliangiam/internal/app/entries"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	httperr "github.com/guliangiam/guliangiam/internal/platform/http"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
	collogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	colmetricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/protobuf/proto"
)

// AddOtlpTrace handles POST /api/v1/entries/traces (OTLP protobuf).
func (h Handler) AddOtlpTrace(w http.ResponseWriter, r *http.Request) {
	body, ok := readOtlpBody(w, r)
	if !ok {
		return
	}

	orgID := authtransport.TrustedOrganizationID(r, r.URL.Query().Get("organizationId"))
	if orgID == "" {
		route.WriteError(w, r, apperrors.NewInvalidArgument("organizationId is required"))
		return
	}
	provider := r.URL.Query().Get("provider")

	if err := h.Service.IngestTrace(r.Context(), orgID, provider, r.RemoteAddr, r.UserAgent(), body); err != nil {
		route.WriteError(w, r, err)
		return
	}

	resp := &coltracepb.ExportTraceServiceResponse{}
	writeProtobufResponse(w, r, resp)
}

// AddOtlpMetrics handles POST /api/v1/entries/metrics (OTLP protobuf).
func (h Handler) AddOtlpMetrics(w http.ResponseWriter, r *http.Request) {
	body, ok := readOtlpBody(w, r)
	if !ok {
		return
	}

	orgID := authtransport.TrustedOrganizationID(r, r.URL.Query().Get("organizationId"))
	if orgID == "" {
		route.WriteError(w, r, apperrors.NewInvalidArgument("organizationId is required"))
		return
	}
	provider := r.URL.Query().Get("provider")

	if err := h.Service.IngestMetrics(r.Context(), orgID, provider, r.RemoteAddr, r.UserAgent(), body); err != nil {
		route.WriteError(w, r, err)
		return
	}

	resp := &colmetricspb.ExportMetricsServiceResponse{}
	writeProtobufResponse(w, r, resp)
}

// AddOtlpLogs handles POST /api/v1/entries/logs (OTLP protobuf).
func (h Handler) AddOtlpLogs(w http.ResponseWriter, r *http.Request) {
	body, ok := readOtlpBody(w, r)
	if !ok {
		return
	}

	orgID := authtransport.TrustedOrganizationID(r, r.URL.Query().Get("organizationId"))
	if orgID == "" {
		route.WriteError(w, r, apperrors.NewInvalidArgument("organizationId is required"))
		return
	}
	provider := r.URL.Query().Get("provider")

	if err := h.Service.IngestLogs(r.Context(), orgID, provider, r.RemoteAddr, r.UserAgent(), body); err != nil {
		route.WriteError(w, r, err)
		return
	}

	resp := &collogspb.ExportLogsServiceResponse{}
	writeProtobufResponse(w, r, resp)
}

// readOtlpBody enforces the protobuf media type and reads the body. Every
// failure is written as the same application envelope (415 / 413 / 400), so
// the three OTLP endpoints cannot answer half in text and half in JSON.
func readOtlpBody(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	if !isProtobufContentType(r) {
		httperr.WriteError(w, r, apperrors.NewUnsupportedMediaType("unsupported media type: expected application/x-protobuf"))
		return nil, false
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		// The bootstrap body wall wraps r.Body in http.MaxBytesReader. A body
		// over the configured cap is 413 request_entity_too_large, not a generic
		// 400; both paths use the same envelope writer.
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			httperr.WriteError(w, r, apperrors.NewRequestEntityTooLargef("request body exceeds the %d byte limit", maxErr.Limit))
			return nil, false
		}
		httperr.WriteError(w, r, apperrors.NewInvalidArgument("failed to read request body"))
		return nil, false
	}
	return body, true
}

// isProtobufContentType checks if the request Content-Type is application/x-protobuf.
// It parses the media type instead of prefix-matching the raw header so a
// parameter order, whitespace or case difference cannot split the three
// endpoints between 415 and a parse attempt.
func isProtobufContentType(r *http.Request) bool {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil {
		return false
	}
	return strings.EqualFold(mediaType, "application/x-protobuf")
}

// writeProtobufResponse serializes a protobuf message and writes it as
// application/x-protobuf.
func writeProtobufResponse(w http.ResponseWriter, r *http.Request, msg proto.Message) {
	writeProtobufResponseWith(w, r, msg, proto.Marshal)
}

// writeProtobufResponseWith exists so the encoding-failure branch is testable
// without a message that can make proto.Marshal fail.
func writeProtobufResponseWith(
	w http.ResponseWriter,
	r *http.Request,
	msg proto.Message,
	marshal func(proto.Message) ([]byte, error),
) {
	data, err := marshal(msg)
	if err != nil {
		httperr.WriteError(w, r, apperrors.NewInternal("failed to encode protobuf response"))
		return
	}
	w.Header().Set("Content-Type", "application/x-protobuf")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// Verify Handler implements OTLP methods at compile time.
var _ = (appentries.Service).IngestTrace

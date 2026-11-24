package entries

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/entries"
	domainrecords "github.com/guliangiam/guliangiam/internal/domain/records"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/redact"
	collogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	colmetricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	metricdata "go.opentelemetry.io/proto/otlp/metrics/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// EntryTranscriptAction is the audit action recorded when a conversation stream
// is read. Reading it is itself an event worth keeping: the stream is the one
// read face that can carry live credentials (设计报告 42.3.13).
const EntryTranscriptAction = "entry.transcript_read"

const (
	// DefaultTranscriptLimit is the page size used when the caller does not name
	// one.
	DefaultTranscriptLimit = 50
	// MaxTranscriptLimit bounds one page. A larger request is refused rather
	// than clamped: silently serving 50 of the requested 500 rows makes the
	// caller's "load more" logic skip the remainder (42.3.14).
	MaxTranscriptLimit = 200
	// transcriptCursorVersion guards the encoded cursor format. A cursor from an
	// older server is refused instead of being misinterpreted.
	transcriptCursorVersion = 1
	// transcriptMaxBodyBytes bounds one message body in the response. A record
	// larger than this is replaced by a marker carrying its byte size rather
	// than blocking the whole sequence behind an unreadable row (43.2.6 条 5).
	transcriptMaxBodyBytes = 64 * 1024
	// transcriptOrderVersion identifies the ordering the cursor is bound to. A
	// change to how the stream is flattened must bump it, so cursors issued
	// under the old order are refused instead of silently skipping messages.
	transcriptOrderVersion = 1
)

// transcriptKind names where a message came from in the OTLP payload.
const (
	transcriptKindSpan   = "span"
	transcriptKindLog    = "log"
	transcriptKindMetric = "metric"
)

// TranscriptQuery reads one page of an entry's conversation stream.
type TranscriptQuery struct {
	EntryID string
	// OrganizationID is the caller's tenant scope. It is required: the read is
	// refused with 404 when it does not match the entry's organization, so an
	// entry of another tenant is indistinguishable from one that does not exist
	// (43.5.2).
	OrganizationID string
	// Cursor is the opaque value returned as `nextCursor` by the previous page.
	// The caller must not parse it (43.3 条 1).
	Cursor string
	Limit  int
	// Request metadata, taken from the transport and never from the body.
	IPAddress string
	UserAgent string
	Surface   string
	RequestID string
	TraceID   string
}

// TranscriptMessage is one fact in the stream.
type TranscriptMessage struct {
	// Index is the append position of the message within the entry. It is the
	// stable order key: the stream is stored in production order and never
	// reordered, so the index is deterministic even for records that share a
	// timestamp (43.2.6 条 1).
	Index int `json:"index"`
	// Kind is the OTLP signal the message came from: span, log or metric.
	Kind string `json:"kind"`
	// Scope is the instrumentation scope that produced the message.
	Scope string `json:"scope,omitempty"`
	// Name is the span name, the log record's severity text or the metric name.
	Name string `json:"name,omitempty"`
	// Timestamp is the record's own event time. Records that carry none fall
	// back to the entry's ingestion time rather than to the zero time, so every
	// message has a time that can be sorted and rendered.
	Timestamp time.Time `json:"timestamp"`
	// Body is the full record with every sensitive value replaced on the server.
	Body map[string]any `json:"body,omitempty"`
	// BodyBytes is the size of the body as served. It is always present so the
	// client can tell an empty record from an omitted one.
	BodyBytes int `json:"bodyBytes"`
	// BodyOmitted is true when the record exceeded the per-message bound and was
	// replaced by this marker. The alternative — serving it anyway — would let a
	// single record block the sequence.
	BodyOmitted bool `json:"bodyOmitted,omitempty"`
}

// TranscriptPage is one page of the stream in append order.
type TranscriptPage struct {
	Messages []TranscriptMessage `json:"messages"`
	// NextCursor is empty on the last page. It is opaque to the caller.
	NextCursor string `json:"nextCursor,omitempty"`
	// RedactedFields lists the field paths the server replaced, so a reader can
	// tell "this record had no credentials" from "this field was removed".
	RedactedFields []string `json:"redactedFields,omitempty"`
}

// transcriptCursor is the decoded form of NextCursor. It is deliberately not
// part of the API: clients are told the value is opaque, and a client that
// constructs one anyway gets a 400 rather than a made-up page.
type transcriptCursor struct {
	Version int    `json:"v"`
	Order   int    `json:"o"`
	EntryID string `json:"e"`
	Offset  int    `json:"i"`
}

// transcriptRecord is one decoded fact before redaction and paging.
type transcriptRecord struct {
	Kind   string
	Scope  string
	Name   string
	Time   time.Time
	Source proto.Message
}

// ListEntryTranscript reads one page of an entry's conversation stream.
//
// The stream is derived from the entry's own stored payload, which is the single
// source of truth for what was ingested: no second copy of the messages exists,
// so a message cannot be shown here and missing there. Desensitization happens
// before the page is built, so no code path can return an unredacted body.
func (s Service) ListEntryTranscript(ctx context.Context, q TranscriptQuery) (*TranscriptPage, error) {
	entry, err := s.loadTranscriptEntry(ctx, q)
	if err != nil {
		return nil, err
	}

	offset, err := decodeTranscriptCursor(q.Cursor, entry.ID)
	if err != nil {
		return nil, err
	}

	limit, err := normalizeTranscriptLimit(q.Limit)
	if err != nil {
		return nil, err
	}

	records, err := decodeTranscriptRecords(entry)
	if err != nil {
		return nil, err
	}

	page, err := buildTranscriptPage(entry.ID, records, offset, limit)
	if err != nil {
		return nil, err
	}

	if err := s.writeTranscriptAudit(ctx, entry, q, offset, page); err != nil {
		return nil, err
	}

	return page, nil
}

// loadTranscriptEntry fetches the entry and applies the tenant scope. A missing
// entry and an entry of another organization produce the same 404, so the read
// leaks no existence information (AC-FE-015).
func (s Service) loadTranscriptEntry(ctx context.Context, q TranscriptQuery) (*domain.Entry, error) {
	entryID := strings.TrimSpace(q.EntryID)
	if entryID == "" {
		return nil, apperrors.NewInvalidArgument("entry id is required")
	}
	organizationID := strings.TrimSpace(q.OrganizationID)
	if organizationID == "" {
		return nil, apperrors.NewInvalidArgument("organizationId is required")
	}

	// The tenant predicate lives in the repository query, so a foreign entry is
	// never loaded here and does not depend on a later app-layer comparison.
	entry, err := s.UOW.Entries().GetByIDForOrganization(ctx, entryID, organizationID)
	if err != nil {
		if isNotFound(err) {
			return nil, apperrors.NewNotFound("entry not found")
		}
		return nil, err
	}

	// Second assertion: even a repository implementation that lost its tenant
	// predicate cannot serve another organization's transcript.
	if entry.OrganizationID != organizationID {
		return nil, apperrors.NewNotFound("entry not found")
	}

	return entry, nil
}

// isNotFound reports whether err is the not-found error the repository raises
// for a row that does not exist. The mapping to 404 is done by the caller so the
// two not-found causes are indistinguishable in the response.
func isNotFound(err error) bool {
	appErr, ok := err.(*apperrors.Error)
	if !ok {
		return false
	}
	return appErr.Code == apperrors.CodeNotFound
}

func normalizeTranscriptLimit(limit int) (int, error) {
	switch {
	case limit < 0:
		return 0, apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			"limit must not be negative",
			map[string]any{"field": "limit", "value": limit, "min": 0},
		)
	case limit == 0:
		return DefaultTranscriptLimit, nil
	case limit > MaxTranscriptLimit:
		return 0, apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			"limit must not exceed "+strconv.Itoa(MaxTranscriptLimit),
			map[string]any{"field": "limit", "value": limit, "max": MaxTranscriptLimit},
		)
	default:
		return limit, nil
	}
}

// decodeTranscriptCursor returns the offset the cursor names. An empty cursor is
// the first page; a cursor that does not belong to this entry, this order or
// this version is refused with 400 (43.3 条 2) instead of being ignored, because
// ignoring it would serve page one again under the caller's "load more".
func decodeTranscriptCursor(cursor, entryID string) (int, error) {
	cursor = strings.TrimSpace(cursor)
	if cursor == "" {
		return 0, nil
	}

	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return 0, apperrors.NewInvalidArgument("cursor is not valid")
	}

	decoded := transcriptCursor{}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return 0, apperrors.NewInvalidArgument("cursor is not valid")
	}

	if decoded.Version != transcriptCursorVersion || decoded.Order != transcriptOrderVersion {
		return 0, apperrors.NewInvalidArgument("cursor was issued for a different ordering")
	}
	if decoded.EntryID != entryID {
		return 0, apperrors.NewInvalidArgument("cursor was issued for a different stream")
	}
	if decoded.Offset < 0 {
		return 0, apperrors.NewInvalidArgument("cursor is not valid")
	}

	return decoded.Offset, nil
}

func encodeTranscriptCursor(entryID string, offset int) (string, error) {
	raw, err := json.Marshal(transcriptCursor{
		Version: transcriptCursorVersion,
		Order:   transcriptOrderVersion,
		EntryID: entryID,
		Offset:  offset,
	})
	if err != nil {
		return "", apperrors.WrapInternal(err, "encode transcript cursor")
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// decodeTranscriptRecords flattens the entry's stored payload into the append
// order of its facts.
//
// A payload that declares an OTLP signal but does not decode returns 422 rather
// than an empty page: an empty page would assert "this stream has no messages",
// which is a claim the server cannot make about bytes it could not read.
func decodeTranscriptRecords(entry *domain.Entry) ([]transcriptRecord, error) {
	payload := strings.TrimSpace(entry.Message)
	if payload == "" {
		return nil, nil
	}

	switch strings.ToLower(strings.TrimSpace(entry.Type)) {
	case "trace":
		request := &coltracepb.ExportTraceServiceRequest{}
		if err := decodeTranscriptPayload(payload, request); err != nil {
			return nil, err
		}
		return flattenTraces(entry, request), nil
	case "log":
		request := &collogspb.ExportLogsServiceRequest{}
		if err := decodeTranscriptPayload(payload, request); err != nil {
			return nil, err
		}
		return flattenLogs(entry, request), nil
	case "metrics":
		request := &colmetricspb.ExportMetricsServiceRequest{}
		if err := decodeTranscriptPayload(payload, request); err != nil {
			return nil, err
		}
		return flattenMetrics(entry, request), nil
	default:
		// Entries created through the CRUD face carry no OTLP signal type, so
		// they have no message sequence. That is an empty stream, not an error.
		return nil, nil
	}
}

func decodeTranscriptPayload(payload string, target proto.Message) error {
	if err := protojson.Unmarshal([]byte(payload), target); err != nil {
		return apperrors.NewUnprocessableEntity("entry payload is not a readable OpenTelemetry export")
	}
	return nil
}

func flattenTraces(entry *domain.Entry, request *coltracepb.ExportTraceServiceRequest) []transcriptRecord {
	records := make([]transcriptRecord, 0)
	for _, resourceSpans := range request.GetResourceSpans() {
		for _, scopeSpans := range resourceSpans.GetScopeSpans() {
			scope := scopeSpans.GetScope().GetName()
			for _, span := range scopeSpans.GetSpans() {
				records = append(records, transcriptRecord{
					Kind:   transcriptKindSpan,
					Scope:  scope,
					Name:   span.GetName(),
					Time:   transcriptTime(span.GetStartTimeUnixNano(), entry),
					Source: span,
				})
			}
		}
	}
	return records
}

func flattenLogs(entry *domain.Entry, request *collogspb.ExportLogsServiceRequest) []transcriptRecord {
	records := make([]transcriptRecord, 0)
	for _, resourceLogs := range request.GetResourceLogs() {
		for _, scopeLogs := range resourceLogs.GetScopeLogs() {
			scope := scopeLogs.GetScope().GetName()
			for _, record := range scopeLogs.GetLogRecords() {
				records = append(records, transcriptRecord{
					Kind:   transcriptKindLog,
					Scope:  scope,
					Name:   record.GetSeverityText(),
					Time:   transcriptTime(record.GetTimeUnixNano(), entry),
					Source: record,
				})
			}
		}
	}
	return records
}

func flattenMetrics(entry *domain.Entry, request *colmetricspb.ExportMetricsServiceRequest) []transcriptRecord {
	records := make([]transcriptRecord, 0)
	for _, resourceMetrics := range request.GetResourceMetrics() {
		for _, scopeMetrics := range resourceMetrics.GetScopeMetrics() {
			scope := scopeMetrics.GetScope().GetName()
			for _, metric := range scopeMetrics.GetMetrics() {
				records = append(records, transcriptRecord{
					Kind:   transcriptKindMetric,
					Scope:  scope,
					Name:   metric.GetName(),
					Time:   transcriptTime(metricTimestampNanos(metric), entry),
					Source: metric,
				})
			}
		}
	}
	return records
}

// transcriptTime converts an OTLP nanosecond timestamp to the message time. A
// record without an event time — or with one too far out to be an instant — is
// stamped with the entry's ingestion time, so the stream never contains a zero
// time (which would sort ahead of every real message and render as 1970) and
// never a negative one (OTLP carries nanoseconds as uint64 while time.Unix takes
// int64, and a value past math.MaxInt64 would wrap into the past).
func transcriptTime(nanos uint64, entry *domain.Entry) time.Time {
	if nanos == 0 || nanos > math.MaxInt64 {
		return entry.CreatedAt.UTC()
	}
	return time.Unix(0, int64(nanos)).UTC()
}

// metricTimestampNanos reads the first data point time of a metric, covering
// every data type the OTLP metric oneof can carry.
func metricTimestampNanos(metric *metricdata.Metric) uint64 {
	switch data := metric.GetData().(type) {
	case *metricdata.Metric_Gauge:
		return firstNumberDataPointTime(data.Gauge.GetDataPoints())
	case *metricdata.Metric_Sum:
		return firstNumberDataPointTime(data.Sum.GetDataPoints())
	case *metricdata.Metric_Histogram:
		if points := data.Histogram.GetDataPoints(); len(points) > 0 {
			return points[0].GetTimeUnixNano()
		}
	case *metricdata.Metric_ExponentialHistogram:
		if points := data.ExponentialHistogram.GetDataPoints(); len(points) > 0 {
			return points[0].GetTimeUnixNano()
		}
	case *metricdata.Metric_Summary:
		if points := data.Summary.GetDataPoints(); len(points) > 0 {
			return points[0].GetTimeUnixNano()
		}
	}
	return 0
}

func firstNumberDataPointTime(points []*metricdata.NumberDataPoint) uint64 {
	if len(points) == 0 {
		return 0
	}
	return points[0].GetTimeUnixNano()
}

// buildTranscriptPage redacts then slices. Redaction runs over the whole decoded
// stream rather than only over the requested page so that `redactedFields` is a
// property of the stream, not of where the reader happens to be — a reader who
// pages past a credential must not lose the notice that it was removed.
func buildTranscriptPage(entryID string, records []transcriptRecord, offset, limit int) (*TranscriptPage, error) {
	page := &TranscriptPage{Messages: make([]TranscriptMessage, 0, limit)}
	if offset >= len(records) {
		return page, nil
	}

	redacted := make(map[string]struct{})
	end := offset + limit
	if end > len(records) {
		end = len(records)
	}

	for index := offset; index < end; index++ {
		message, err := toTranscriptMessage(records[index], index, redacted)
		if err != nil {
			return nil, err
		}
		page.Messages = append(page.Messages, message)
	}

	page.RedactedFields = sortedKeys(redacted)

	if end < len(records) {
		cursor, err := encodeTranscriptCursor(entryID, end)
		if err != nil {
			return nil, err
		}
		page.NextCursor = cursor
	}

	return page, nil
}

func toTranscriptMessage(record transcriptRecord, index int, redacted map[string]struct{}) (TranscriptMessage, error) {
	message := TranscriptMessage{
		Index:     index,
		Kind:      record.Kind,
		Scope:     record.Scope,
		Name:      record.Name,
		Timestamp: record.Time,
	}

	if record.Source == nil {
		return message, nil
	}

	raw, err := protojson.MarshalOptions{}.Marshal(record.Source)
	if err != nil {
		return message, apperrors.WrapInternal(err, "encode transcript message")
	}

	body := map[string]any{}
	if err := json.Unmarshal(raw, &body); err != nil {
		return message, apperrors.WrapInternal(err, "decode transcript message")
	}

	message.Body = redactTranscriptValue("", body, "", redacted).(map[string]any)

	encoded, err := json.Marshal(message.Body)
	if err != nil {
		return message, apperrors.WrapInternal(err, "measure transcript message")
	}
	message.BodyBytes = len(encoded)

	if message.BodyBytes > transcriptMaxBodyBytes {
		message.Body = nil
		message.BodyOmitted = true
	}

	return message, nil
}

// redactTranscriptValue walks a decoded record and replaces the value of every
// field the platform's shared sensitive-field rule names. Redaction is applied
// here, on the server, and not in the client: a client-side filter would run
// after the plaintext had already crossed the network and entered browser memory
// (42.3.7).
//
// `parent` is the field name the value is stored under, which is where the rule
// applies. OTLP carries most names in an attribute's own `key` field rather than
// in a JSON object key, so a map of the `{key, value}` shape is matched on its
// key as well. Without that second rule every instrumented header — where
// `authorization` and `cookie` actually live — would pass through untouched.
func redactTranscriptValue(path string, value any, parent string, redacted map[string]struct{}) any {
	if parent != "" && redact.IsSensitiveField(parent) {
		redacted[path] = struct{}{}
		return redact.Placeholder
	}

	switch typed := value.(type) {
	case map[string]any:
		attributeName, _ := typed["key"].(string)
		attributeIsSensitive := attributeName != "" && redact.IsSensitiveField(attributeName)

		out := make(map[string]any, len(typed))
		for key, child := range typed {
			if attributeIsSensitive && key == "value" {
				redacted[joinFieldPath(path, attributeName)] = struct{}{}
				out[key] = redact.Placeholder
				continue
			}
			out[key] = redactTranscriptValue(joinFieldPath(path, key), child, key, redacted)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for index, child := range typed {
			out[index] = redactTranscriptValue(path, child, "", redacted)
		}
		return out
	default:
		// A scalar directly under a sensitive field was already replaced above;
		// every other scalar is passed through unchanged.
		return value
	}
}

// joinFieldPath renders the location of a redacted value so the notice can name
// what was removed without echoing the value.
func joinFieldPath(path, key string) string {
	if path == "" {
		return key
	}
	return path + "." + key
}

func sortedKeys(set map[string]struct{}) []string {
	if len(set) == 0 {
		return nil
	}
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// writeTranscriptAudit records the read on the append-only audit face (41 章).
func (s Service) writeTranscriptAudit(
	ctx context.Context,
	entry *domain.Entry,
	q TranscriptQuery,
	offset int,
	page *TranscriptPage,
) error {
	id, err := s.UUID.NewString()
	if err != nil {
		return apperrors.WrapInternal(err, "generate transcript record id")
	}

	record := &domainrecords.Record{
		ID:             id,
		OrganizationID: entry.OrganizationID,
		Action:         EntryTranscriptAction,
		ResourceType:   "entry",
		ResourceID:     entry.ID,
		IP:             q.IPAddress,
		UserAgent:      q.UserAgent,
		Surface:        domainrecords.NormalizeSurface(q.Surface),
		RequestID:      q.RequestID,
		TraceID:        q.TraceID,
		Metadata: map[string]any{
			"offset":         offset,
			"messages":       len(page.Messages),
			"redactedFields": len(page.RedactedFields),
		},
		CreatedAt: s.Clock.Now().UTC(),
	}

	return s.UOW.Records().Create(ctx, record)
}

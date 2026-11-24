package entries

import (
	"context"
	"fmt"
	"strings"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/entries"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	collogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	colmetricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// IngestTrace accepts an OTLP ExportTraceServiceRequest protobuf body,
// serializes it to JSON, and stores it as an Entry with type "trace".
func (s Service) IngestTrace(ctx context.Context, organizationID, provider, clientIP, userAgent string, body []byte) error {
	req := &coltracepb.ExportTraceServiceRequest{}
	if err := proto.Unmarshal(body, req); err != nil {
		return apperrors.WrapInvalidArgument(err, "invalid OTLP trace payload")
	}

	jsonBytes, err := protojson.Marshal(req)
	if err != nil {
		return apperrors.WrapInternal(err, "marshal trace to json")
	}

	return s.createOtlpEntry(ctx, organizationID, provider, "trace", clientIP, userAgent, string(jsonBytes))
}

// IngestMetrics accepts an OTLP ExportMetricsServiceRequest protobuf body,
// serializes it to JSON, and stores it as an Entry with type "metrics".
func (s Service) IngestMetrics(ctx context.Context, organizationID, provider, clientIP, userAgent string, body []byte) error {
	req := &colmetricspb.ExportMetricsServiceRequest{}
	if err := proto.Unmarshal(body, req); err != nil {
		return apperrors.WrapInvalidArgument(err, "invalid OTLP metrics payload")
	}

	jsonBytes, err := protojson.Marshal(req)
	if err != nil {
		return apperrors.WrapInternal(err, "marshal metrics to json")
	}

	return s.createOtlpEntry(ctx, organizationID, provider, "metrics", clientIP, userAgent, string(jsonBytes))
}

// IngestLogs accepts an OTLP ExportLogsServiceRequest protobuf body,
// serializes it to JSON, and stores it as an Entry with type "log".
func (s Service) IngestLogs(ctx context.Context, organizationID, provider, clientIP, userAgent string, body []byte) error {
	req := &collogspb.ExportLogsServiceRequest{}
	if err := proto.Unmarshal(body, req); err != nil {
		return apperrors.WrapInvalidArgument(err, "invalid OTLP logs payload")
	}

	jsonBytes, err := protojson.Marshal(req)
	if err != nil {
		return apperrors.WrapInternal(err, "marshal logs to json")
	}

	return s.createOtlpEntry(ctx, organizationID, provider, "log", clientIP, userAgent, string(jsonBytes))
}

// createOtlpEntry creates a single Entry record from OTLP ingestion data.
func (s Service) createOtlpEntry(ctx context.Context, organizationID, provider, entryType, clientIP, userAgent, message string) error {
	if strings.TrimSpace(organizationID) == "" {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	id, err := s.UUID.NewString()
	if err != nil {
		return err
	}

	now := s.Clock.Now()
	name := fmt.Sprintf("otlp-%s-%s", entryType, id[:8])

	entry := &domain.Entry{
		ID:             id,
		OrganizationID: organizationID,
		Name:           name,
		DisplayName:    fmt.Sprintf("OTLP %s import", entryType),
		Provider:       provider,
		Type:           entryType,
		ClientIP:       clientIP,
		UserAgent:      userAgent,
		Message:        message,
		CreatedAt:      now,
		UpdatedAt:      now,
		Revision:       1,
	}

	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		if err := tx.Entries().Create(ctx, entry); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx,
			ctx,
			entry.OrganizationID,
			"entries.create",
			entry.ID,
			"entries.create",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}

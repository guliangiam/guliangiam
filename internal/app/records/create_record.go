package records

import (
	"context"

	domain "github.com/guliangiam/guliangiam/internal/domain/records"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// CreateRecordCommand is the command for creating an audit record.
type CreateRecordCommand struct {
	OrganizationID string
	ActorID        string
	Action         string
	ResourceType   string
	ResourceID     string
	IP             string
	UserAgent      string
	// Surface / RequestID / TraceID come from the transport layer's request
	// metadata (手册 12.5) and are never client supplied.
	Surface   string
	RequestID string
	TraceID   string
	Metadata  map[string]any
}

// CreateRecord creates a new audit record.
func (s Service) CreateRecord(ctx context.Context, command CreateRecordCommand) (*RecordDTO, error) {
	if err := validateCreateRecordCommand(command); err != nil {
		return nil, err
	}

	id, err := s.UUID.NewString()
	if err != nil {
		return nil, apperrors.WrapInternal(err, "generate record id")
	}

	now := s.Clock.Now()
	record := &domain.Record{
		ID:             id,
		OrganizationID: command.OrganizationID,
		ActorID:        command.ActorID,
		Action:         command.Action,
		ResourceType:   command.ResourceType,
		ResourceID:     command.ResourceID,
		IP:             command.IP,
		UserAgent:      command.UserAgent,
		Surface:        domain.NormalizeSurface(command.Surface),
		RequestID:      command.RequestID,
		TraceID:        command.TraceID,
		Metadata:       command.Metadata,
		CreatedAt:      now,
	}

	if err := s.UOW.Records().Create(ctx, record); err != nil {
		return nil, apperrors.WrapInternal(err, "create record")
	}

	return toRecordDTO(record), nil
}

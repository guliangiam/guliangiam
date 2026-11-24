package privacy

import (
	"context"

	domain "github.com/guliangiam/guliangiam/internal/domain/privacy"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

// ConsentService implements the consent version chain API (AC-PRIV-002):
// grants append a new version, withdrawal stamps the latest version, and both
// paths write an audit record + outbox event in the same transaction.
type ConsentService struct {
	Service
}

func NewConsentService(ck clock.Clock, generator platformuuid.Generator, runner TransactionRunner) ConsentService {
	return ConsentService{Service: NewService(ck, generator, runner)}
}

type ListConsentsQuery struct {
	OrganizationID string
	SubjectType    string
	SubjectID      string
	Purpose        string
	Limit          int
}

func (s ConsentService) List(ctx context.Context, query ListConsentsQuery) ([]*domain.ConsentRecord, error) {
	var records []*domain.ConsentRecord
	err := s.Runner.WithinTransaction(ctx, func(ctx context.Context, tx Tx) error {
		var err error
		records, err = tx.Consents().List(ctx, query.OrganizationID, domain.ConsentQuery{
			SubjectType: query.SubjectType,
			SubjectID:   query.SubjectID,
			Purpose:     query.Purpose,
			Limit:       query.Limit,
		})
		return err
	})
	return records, err
}

type GrantConsentCommand struct {
	OrganizationID string
	ActorID        string
	SubjectType    string
	SubjectID      string
	Purpose        string
}

// Grant appends the next consent version for (subject, purpose).
func (s ConsentService) Grant(ctx context.Context, command GrantConsentCommand) (*domain.ConsentRecord, error) {
	if command.OrganizationID == "" {
		return nil, apperrors.NewInvalidArgument("organizationId is required")
	}
	if command.SubjectType == "" {
		command.SubjectType = domain.SubjectTypeUser
	}
	now := s.Clock.Now().UTC()
	record := &domain.ConsentRecord{
		OrganizationID: command.OrganizationID,
		SubjectType:    command.SubjectType,
		SubjectID:      command.SubjectID,
		Purpose:        command.Purpose,
		GrantedAt:      now,
		CreatedAt:      now,
		UpdatedAt:      now,
		Revision:       1,
	}
	if err := record.ValidateSubject(); err != nil {
		return nil, err
	}
	id, err := s.UUID.NewString()
	if err != nil {
		return nil, err
	}
	record.ID = id

	err = s.Runner.WithinTransaction(ctx, func(ctx context.Context, tx Tx) error {
		// BE-D-22c：先取事务级 subject 锁再读最新版本，防止并发 Grant 重号。
		if err := tx.Consents().LockSubject(ctx, command.OrganizationID, command.SubjectType, command.SubjectID, command.Purpose); err != nil {
			return err
		}
		latest, err := tx.Consents().Latest(ctx, command.OrganizationID, command.SubjectType, command.SubjectID, command.Purpose)
		if err != nil {
			if typed, ok := err.(*apperrors.Error); !ok || typed.Code != apperrors.CodeNotFound {
				return err
			}
			record.Version = 1
		} else {
			record.Version = latest.Version + 1
		}
		if err := record.Validate(); err != nil {
			return err
		}
		if err := tx.Consents().Create(ctx, record); err != nil {
			return err
		}
		return tx.EmbedAudit(ctx, AuditEvent{
			OrganizationID: command.OrganizationID,
			ActorID:        command.ActorID,
			Action:         "privacy.consent.granted",
			ResourceType:   "consent_record",
			ResourceID:     record.ID,
			AggregateType:  "consent_record",
			EventType:      "privacy.consent.granted",
			Payload: map[string]any{
				"purpose":     record.Purpose,
				"version":     record.Version,
				"subjectType": record.SubjectType,
			},
		})
	})
	if err != nil {
		return nil, err
	}
	return record, nil
}

type WithdrawConsentCommand struct {
	OrganizationID string
	ActorID        string
	SubjectType    string
	SubjectID      string
	Purpose        string
}

// Withdraw stamps withdrawn_at on the latest version; history is preserved and
// an idempotent second call returns NotFound (no active consent).
func (s ConsentService) Withdraw(ctx context.Context, command WithdrawConsentCommand) (*domain.ConsentRecord, error) {
	if command.OrganizationID == "" {
		return nil, apperrors.NewInvalidArgument("organizationId is required")
	}
	if command.SubjectType == "" {
		command.SubjectType = domain.SubjectTypeUser
	}
	if command.SubjectID == "" || command.Purpose == "" {
		return nil, apperrors.NewInvalidArgument("subjectId and purpose are required")
	}
	now := s.Clock.Now().UTC()
	var record *domain.ConsentRecord
	err := s.Runner.WithinTransaction(ctx, func(ctx context.Context, tx Tx) error {
		var err error
		record, err = tx.Consents().WithdrawLatest(ctx, command.OrganizationID, command.SubjectType, command.SubjectID, command.Purpose, now)
		if err != nil {
			return err
		}
		return tx.EmbedAudit(ctx, AuditEvent{
			OrganizationID: command.OrganizationID,
			ActorID:        command.ActorID,
			Action:         "privacy.consent.withdrawn",
			ResourceType:   "consent_record",
			ResourceID:     record.ID,
			AggregateType:  "consent_record",
			EventType:      "privacy.consent.withdrawn",
			Payload: map[string]any{
				"purpose":   record.Purpose,
				"version":   record.Version,
				"withdrawn": true,
			},
		})
	})
	if err != nil {
		return nil, err
	}
	return record, nil
}

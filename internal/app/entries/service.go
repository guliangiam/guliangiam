package entries

import (
	"context"
	"strings"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/entries"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

type Service struct {
	Clock clock.Clock
	UUID  platformuuid.Generator
	UOW   appuow.AuditUoW
}

func NewService(clk clock.Clock, uuidGenerator platformuuid.Generator, unitOfWork appuow.UnitOfWork) Service {
	return Service{Clock: clk, UUID: uuidGenerator, UOW: unitOfWork}
}

type ListEntriesQuery struct {
	Page, PageSize                             int
	OrganizationID, Keyword, Type, Application string
}
type CreateEntryCommand struct {
	OrganizationID, Name, DisplayName, Provider, Application, Type, ClientIP, UserAgent, Message string
}
type PatchEntryCommand struct {
	ID             string
	OrganizationID string
	DisplayName    *string
	Message        *string
	Revision       int64
}

type EntryListItem struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organizationId"`
	Name           string    `json:"name"`
	DisplayName    string    `json:"displayName"`
	Type           string    `json:"type,omitempty"`
	Application    string    `json:"application,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
}

// EntryDetail deliberately carries no message body: the stored payload is the
// un-redacted ingest artifact, and serving it here would be the "view raw
// content" endpoint that 42.3.13 条 6 forbids. Message content has exactly one
// read face, the redacted transcript (TranscriptService), so there is no second
// implementation of the redaction rule to drift from 42.3.7.
type EntryDetail struct {
	EntryListItem
	Provider  string `json:"provider,omitempty"`
	ClientIP  string `json:"clientIp,omitempty"`
	UserAgent string `json:"userAgent,omitempty"`
	Revision  int64  `json:"revision"`
}

func toListItem(e *domain.Entry) EntryListItem {
	return EntryListItem{ID: e.ID, OrganizationID: e.OrganizationID, Name: e.Name, DisplayName: e.DisplayName,
		Type: e.Type, Application: e.Application, CreatedAt: e.CreatedAt}
}
func toDetail(e *domain.Entry) EntryDetail {
	return EntryDetail{EntryListItem: toListItem(e), Provider: e.Provider, ClientIP: e.ClientIP,
		UserAgent: e.UserAgent, Revision: e.Revision}
}

func (s Service) ListEntries(ctx context.Context, q ListEntriesQuery) ([]EntryListItem, int64, error) {
	items, total, err := s.UOW.Entries().List(ctx, domain.ListQuery{Page: q.Page, PageSize: q.PageSize, OrganizationID: q.OrganizationID, Keyword: q.Keyword, Type: q.Type, Application: q.Application})
	if err != nil {
		return nil, 0, err
	}
	r := make([]EntryListItem, 0, len(items))
	for _, i := range items {
		r = append(r, toListItem(i))
	}
	return r, total, nil
}
func requireOrganizationID(organizationID string) error {
	if strings.TrimSpace(organizationID) == "" {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	return nil
}

func (s Service) GetEntry(ctx context.Context, organizationID, id string) (*EntryDetail, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	// 租户谓词在仓储层显式生效；MySQL 无 RLS，不能依赖数据库兜底。
	e, err := s.UOW.Entries().GetByIDForOrganization(ctx, id, organizationID)
	if err != nil {
		return nil, err
	}
	d := toDetail(e)
	return &d, nil
}
func (s Service) CreateEntry(ctx context.Context, c CreateEntryCommand) error {
	if strings.TrimSpace(c.OrganizationID) == "" {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	if strings.TrimSpace(c.Name) == "" {
		return apperrors.NewInvalidArgument("name is required")
	}
	id, err := s.UUID.NewString()
	if err != nil {
		return err
	}
	now := s.Clock.Now()
	e := &domain.Entry{ID: id, OrganizationID: c.OrganizationID, Name: c.Name, DisplayName: c.DisplayName,
		Provider: c.Provider, Application: c.Application, Type: c.Type, ClientIP: c.ClientIP,
		UserAgent: c.UserAgent, Message: c.Message, CreatedAt: now, UpdatedAt: now, Revision: 1}
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		if err := tx.Entries().Create(ctx, e); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx,
			ctx,
			e.OrganizationID,
			"entries.create",
			e.ID,
			"entries.create",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}
func (s Service) PatchEntry(ctx context.Context, c PatchEntryCommand) error {
	if err := requireOrganizationID(c.OrganizationID); err != nil {
		return err
	}
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		e, err := tx.Entries().GetByIDForOrganization(ctx, c.ID, c.OrganizationID)
		if err != nil {
			return err
		}
		if c.Revision > 0 && e.Revision != c.Revision {
			return apperrors.NewConflict("entry revision mismatch")
		}
		if c.DisplayName != nil {
			e.DisplayName = *c.DisplayName
		}
		if c.Message != nil {
			e.Message = *c.Message
		}
		e.UpdatedAt = s.Clock.Now()
		e.Revision++
		if err := tx.Entries().Update(ctx, e); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx,
			ctx,
			e.OrganizationID,
			"entries.update",
			e.ID,
			"entries.update",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}
func (s Service) DeleteEntry(ctx context.Context, organizationID, id string) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		e, err := tx.Entries().GetByIDForOrganization(ctx, id, organizationID)
		if err != nil {
			return err
		}
		if err := tx.Entries().Delete(ctx, id, organizationID); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx,
			ctx,
			e.OrganizationID,
			"entries.delete",
			id,
			"entries.delete",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}

// 安全边界：PEP 中间件（APIAuthorizationMiddleware）已通过 Casbin 验证了权限。
// RLS 提升只在 service 内部生效（事务内 LOCAL set_config），事务结束自动清除。

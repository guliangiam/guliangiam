package agents

import (
	"context"
	"strings"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/agents"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

type Service struct {
	Clock clock.Clock
	UUID  platformuuid.Generator
	UOW   appuow.InfraUoW
}

// 安全边界：PEP 中间件（APIAuthorizationMiddleware）已通过 Casbin 验证了权限。
// RLS 提升只在 service 内部生效（事务内 LOCAL set_config），事务结束自动清除。
func NewService(clk clock.Clock, uuidGenerator platformuuid.Generator, unitOfWork appuow.InfraUoW) Service {
	return Service{Clock: clk, UUID: uuidGenerator, UOW: unitOfWork}
}

type ListAgentsQuery struct {
	Page, PageSize          int
	OrganizationID, Keyword string
	IsEnabled               *bool
}
type CreateAgentCommand struct {
	OrganizationID, Name, DisplayName, Type, Host, Url, Token, ApplicationID string
	Port                                                                     int
	IsEnabled                                                                *bool
}
type PatchAgentCommand struct {
	ID                                           string
	DisplayName, Host, Url, Token, ApplicationID *string
	Port                                         *int
	IsEnabled                                    *bool
	Revision                                     int64
}

// requireOrganizationID fails fast when a caller reaches the app layer without
// a trusted tenant. HTTP handlers resolve this via TrustedOrganizationID; the
// guard keeps non-HTTP callers from degrading to a cross-tenant lookup.
func requireOrganizationID(organizationID string) error {
	if strings.TrimSpace(organizationID) == "" {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	return nil
}

type AgentListItem struct {
	ID             string     `json:"id"`
	OrganizationID string     `json:"organizationId"`
	Name           string     `json:"name"`
	DisplayName    string     `json:"displayName"`
	Type           string     `json:"type"`
	Host           string     `json:"host"`
	Port           int        `json:"port"`
	Url            string     `json:"url"`
	Token          string     `json:"token,omitempty"`
	ApplicationID  string     `json:"applicationId,omitempty"`
	IsEnabled      bool       `json:"isEnabled"`
	LastPingAt     *time.Time `json:"lastPingAt,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}
type AgentDetail struct {
	AgentListItem
	Revision int64 `json:"revision"`
}

func toListItem(a *domain.Agent) AgentListItem {
	token := a.Token
	if len(token) > 8 {
		token = token[:4] + "****" + token[len(token)-4:]
	}
	return AgentListItem{ID: a.ID, OrganizationID: a.OrganizationID, Name: a.Name, DisplayName: a.DisplayName,
		Type: a.Type, Host: a.Host, Port: a.Port, Url: a.Url, Token: token, ApplicationID: a.ApplicationID,
		IsEnabled:  a.IsEnabled,
		LastPingAt: a.LastPingAt, CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt}
}
func toDetail(a *domain.Agent) AgentDetail {
	return AgentDetail{AgentListItem: toListItem(a), Revision: a.Revision}
}

func (s Service) ListAgents(ctx context.Context, q ListAgentsQuery) ([]AgentListItem, int64, error) {
	items, total, err := s.UOW.Agents().List(ctx, domain.ListQuery(q))
	if err != nil {
		return nil, 0, err
	}
	r := make([]AgentListItem, 0, len(items))
	for _, i := range items {
		r = append(r, toListItem(i))
	}
	return r, total, nil
}
func (s Service) GetAgent(ctx context.Context, organizationID, id string) (*AgentDetail, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	a, err := s.UOW.Agents().GetByID(ctx, id, organizationID)
	if err != nil {
		return nil, err
	}
	d := toDetail(a)
	return &d, nil
}
func (s Service) CreateAgent(ctx context.Context, c CreateAgentCommand) error {
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
	isEnabled := true
	if c.IsEnabled != nil {
		isEnabled = *c.IsEnabled
	}
	a := &domain.Agent{ID: id, OrganizationID: c.OrganizationID, Name: c.Name, DisplayName: c.DisplayName,
		Type: c.Type, Host: c.Host, Port: c.Port, Url: c.Url, Token: c.Token, ApplicationID: c.ApplicationID,
		IsEnabled: isEnabled,
		CreatedAt: now, UpdatedAt: now, Revision: 1}
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		if err := tx.Agents().Create(ctx, a); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx, ctx,
			"", "", "", "", "",
			a.OrganizationID, "",
			"agents.create", "agents", a.ID,
			"agents", "agents.create",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}
func (s Service) PatchAgent(ctx context.Context, organizationID string, c PatchAgentCommand) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// Ownership is decided before any mutation: a foreign or missing id
		// returns NotFound and leaves no write and no audit record behind.
		a, err := tx.Agents().GetByID(ctx, c.ID, organizationID)
		if err != nil {
			return err
		}
		if c.Revision > 0 && a.Revision != c.Revision {
			return apperrors.NewConflict("agent revision mismatch")
		}
		if c.DisplayName != nil {
			a.DisplayName = *c.DisplayName
		}
		if c.Host != nil {
			a.Host = *c.Host
		}
		if c.Port != nil {
			a.Port = *c.Port
		}
		if c.Url != nil {
			a.Url = *c.Url
		}
		// Token intentionally excluded from Patch — agent authentication secrets
		// are server-generated at create time and MUST NOT be client-overwritable
		// on PATCH. Rotation happens through a dedicated regenerate endpoint.
		if c.ApplicationID != nil {
			a.ApplicationID = *c.ApplicationID
		}
		if c.IsEnabled != nil {
			a.IsEnabled = *c.IsEnabled
		}
		a.UpdatedAt = s.Clock.Now()
		a.Revision++
		if err := tx.Agents().Update(ctx, a); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx, ctx,
			"", "", "", "", "",
			a.OrganizationID, "",
			"agents.update", "agents", a.ID,
			"agents", "agents.update",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}
func (s Service) DeleteAgent(ctx context.Context, organizationID, id string) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// Read first so a foreign id is rejected before the delete and the
		// audit row carries the entity's real organization.
		a, err := tx.Agents().GetByID(ctx, id, organizationID)
		if err != nil {
			return err
		}

		if err := tx.Agents().Delete(ctx, id, organizationID); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx, ctx,
			"", "", "", "", "",
			a.OrganizationID, "",
			"agents.delete", "agents", id,
			"agents", "agents.delete",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}

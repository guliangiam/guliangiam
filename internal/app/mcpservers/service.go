package mcpservers

import (
	"context"
	"strings"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domainauth "github.com/guliangiam/guliangiam/internal/domain/auth"
	domain "github.com/guliangiam/guliangiam/internal/domain/mcpservers"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

type Service struct {
	Clock clock.Clock
	UUID  platformuuid.Generator
	// UOW is the full unit of work: the token mint resolves the target user
	// (a cross-resource membership check) and records the audit decision,
	// which the InfraUoW subset cannot express.
	UOW  appuow.UnitOfWork
	MCP  MCPProxy
	JWKS domainauth.JWKSProvider
}

// MCPProxy is an interface for the MCP proxy functionality.
type MCPProxy interface {
	SyncToolsFromServer(ctx context.Context, serverID string) error
	ListTools(ctx context.Context) []domain.Tool
	CallTool(ctx context.Context, serverID, toolName string, arguments map[string]any) (any, error)
}

func NewService(clk clock.Clock, uuidGenerator platformuuid.Generator, unitOfWork appuow.UnitOfWork) Service {
	return Service{Clock: clk, UUID: uuidGenerator, UOW: unitOfWork}
}

func NewServiceWithDeps(clk clock.Clock, uuidGenerator platformuuid.Generator, unitOfWork appuow.UnitOfWork, mcp MCPProxy, jwks domainauth.JWKSProvider) Service {
	return Service{Clock: clk, UUID: uuidGenerator, UOW: unitOfWork, MCP: mcp, JWKS: jwks}
}

type ListMcpServersQuery struct {
	Page, PageSize          int
	OrganizationID, Keyword string
	IsEnabled               *bool
}

// ToolDTO represents an MCP tool in the application layer, decoupling from domain types.
type ToolDTO struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	InputSchema map[string]interface{} `json:"inputSchema,omitempty"`
	IsAllowed   bool                   `json:"isAllowed"`
}

func (t ToolDTO) toDomain() domain.Tool {
	return domain.Tool{
		Name:        t.Name,
		Description: t.Description,
		InputSchema: t.InputSchema,
		IsAllowed:   t.IsAllowed,
	}
}

type CreateMcpServerCommand struct {
	OrganizationID, Name, DisplayName, URL, Token, Application string
	Tools                                                      []ToolDTO
	IsEnabled                                                  bool
}
type PatchMcpServerCommand struct {
	ID                                   string
	DisplayName, URL, Token, Application *string
	Tools                                []ToolDTO
	IsEnabled                            *bool
	Revision                             int64
}

type McpServerListItem struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organizationId"`
	Name           string    `json:"name"`
	DisplayName    string    `json:"displayName"`
	URL            string    `json:"url,omitempty"`
	Application    string    `json:"application,omitempty"`
	IsEnabled      bool      `json:"isEnabled"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}
type McpServerDetail struct {
	McpServerListItem
	// Token is write-only: the detail response carries only TokenSet/TokenMasked.
	// The plaintext stays server-side (repository reads decrypt it for internal
	// callers); every Listing/DTO boundary must not re-introduce the value.
	TokenSet    bool          `json:"tokenSet"`
	TokenMasked string        `json:"tokenMasked,omitempty"`
	Tools       []domain.Tool `json:"tools,omitempty"`
	Revision    int64         `json:"revision"`
}

func toListItem(s *domain.McpServer) McpServerListItem {
	return McpServerListItem{ID: s.ID, OrganizationID: s.OrganizationID, Name: s.Name, DisplayName: s.DisplayName,
		URL: s.URL, Application: s.Application, IsEnabled: s.IsEnabled, CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt}
}
func toDetail(s *domain.McpServer) McpServerDetail {
	tools := make([]domain.Tool, len(s.Tools))
	copy(tools, s.Tools)
	return McpServerDetail{McpServerListItem: toListItem(s), TokenSet: s.Token != "", TokenMasked: maskToken(s.Token), Tools: tools, Revision: s.Revision}
}

// maskToken returns first 2 + *** + last 2 characters, or empty string.
// Short tokens (< 6 chars) are fully masked; the plaintext is never returned.
func maskToken(token string) string {
	if token == "" {
		return ""
	}
	if len(token) < 6 {
		return "******"
	}
	return token[:2] + "***" + token[len(token)-2:]
}

func (s Service) ListMcpServers(ctx context.Context, q ListMcpServersQuery) ([]McpServerListItem, int64, error) {
	items, total, err := s.UOW.McpServers().List(ctx, domain.ListQuery{Page: q.Page, PageSize: q.PageSize, OrganizationID: q.OrganizationID, Keyword: q.Keyword, IsEnabled: q.IsEnabled})
	if err != nil {
		return nil, 0, err
	}
	r := make([]McpServerListItem, 0, len(items))
	for _, i := range items {
		r = append(r, toListItem(i))
	}
	return r, total, nil
}
func (s Service) GetMcpServer(ctx context.Context, organizationID, id string) (*McpServerDetail, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	srv, err := s.UOW.McpServers().GetByIDForOrganization(ctx, id, organizationID)
	if err != nil {
		return nil, err
	}
	d := toDetail(srv)
	return &d, nil
}
func (s Service) CreateMcpServer(ctx context.Context, c CreateMcpServerCommand) error {
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
	tools := make([]domain.Tool, len(c.Tools))
	for i, t := range c.Tools {
		tools[i] = t.toDomain()
	}
	srv := &domain.McpServer{ID: id, OrganizationID: c.OrganizationID, Name: c.Name, DisplayName: c.DisplayName,
		URL: c.URL, Token: c.Token, Application: c.Application, Tools: tools, IsEnabled: c.IsEnabled,
		CreatedAt: now, UpdatedAt: now, Revision: 1}
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		if err := tx.McpServers().Create(ctx, srv); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx, ctx,
			"", "", "", "", "",
			srv.OrganizationID, "",
			"mcpservers.create", "mcpservers", srv.ID,
			"mcpservers", "mcpservers.create",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}
func (s Service) PatchMcpServer(ctx context.Context, organizationID string, c PatchMcpServerCommand) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// 事务内先做 scoped 读取；跨租户在写与审计之前就返回 NotFound。
		srv, err := tx.McpServers().GetByIDForOrganization(ctx, c.ID, organizationID)
		if err != nil {
			return err
		}
		if c.Revision > 0 && srv.Revision != c.Revision {
			return apperrors.NewConflict("mcp server revision mismatch")
		}
		if c.DisplayName != nil {
			srv.DisplayName = *c.DisplayName
		}
		if c.URL != nil {
			srv.URL = *c.URL
		}
		if c.Token != nil {
			srv.Token = *c.Token
		}
		if c.Application != nil {
			srv.Application = *c.Application
		}
		if c.Tools != nil {
			srv.Tools = make([]domain.Tool, len(c.Tools))
			for i, t := range c.Tools {
				srv.Tools[i] = t.toDomain()
			}
		}
		if c.IsEnabled != nil {
			srv.IsEnabled = *c.IsEnabled
		}
		srv.UpdatedAt = s.Clock.Now()
		srv.Revision++
		if err := tx.McpServers().Update(ctx, srv); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx, ctx,
			"", "", "", "", "",
			srv.OrganizationID, "",
			"mcpservers.update", "mcpservers", srv.ID,
			"mcpservers", "mcpservers.update",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}
func (s Service) DeleteMcpServer(ctx context.Context, organizationID, id string) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// Read first so a foreign id is rejected before the delete and the
		// audit row carries the entity's real organization.
		srv, err := tx.McpServers().GetByIDForOrganization(ctx, id, organizationID)
		if err != nil {
			return err
		}
		if err := tx.McpServers().Delete(ctx, id, srv.OrganizationID); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx, ctx,
			"", "", "", "", "",
			srv.OrganizationID, "",
			"mcpservers.delete", "mcpservers", id,
			"mcpservers", "mcpservers.delete",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}

// 安全边界：PEP 中间件（APIAuthorizationMiddleware）已通过 Casbin 验证了权限。
// RLS 提升只在 service 内部生效（事务内 LOCAL set_config），事务结束自动清除。

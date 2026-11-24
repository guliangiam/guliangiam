// Package workload 提供 ADR-028 Federation Trust 的 HTTP handler（admin API）。
// 与 internal/domain/workload.Repository 接口对接。
package workload

import (
	"context"
	"net/http"
	"strings"

	appworkload "github.com/guliangiam/guliangiam/internal/app/workload"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Handler 依赖注入 domain/workload.Repository。
type Handler struct {
	Repo appworkload.Repository
}

// NewHandler 构造 workload admin API handler。
func NewHandler(repo appworkload.Repository) *Handler {
	return &Handler{Repo: repo}
}

// ── DTO ────────────────────────────────────────────────────────

// CreateTrustRequest 是 admin 创建 Federation Trust 的请求体。
type CreateTrustRequest struct {
	PoolID   string   `json:"poolId"`
	Name     string   `json:"name"`
	Issuer   string   `json:"issuer"`
	JWKSURI  string   `json:"jwksUri"`
	Audience []string `json:"audience"`
	Subjects []string `json:"subjects"`
}

// TrustResponse 是 Federation Trust 的响应体。
type TrustResponse struct {
	ID        string   `json:"id"`
	OrgID     string   `json:"organizationId"`
	PoolID    string   `json:"poolId"`
	Name      string   `json:"name"`
	Issuer    string   `json:"issuer"`
	JWKSURI   string   `json:"jwksUri"`
	Audience  []string `json:"audience"`
	Subjects  []string `json:"subjects"`
	Status    string   `json:"status"`
	CreatedAt string   `json:"createdAt"`
	UpdatedAt string   `json:"updatedAt"`
	Revision  int64    `json:"revision"`
}

func trustToResponse(t *appworkload.FederationTrust) TrustResponse {
	return TrustResponse{
		ID:        t.ID,
		OrgID:     t.OrganizationID,
		PoolID:    t.PoolID,
		Name:      t.Name,
		Issuer:    t.Issuer,
		JWKSURI:   t.JWKSURI,
		Audience:  t.Audience,
		Subjects:  t.Subjects,
		Status:    string(t.Status),
		CreatedAt: t.CreatedAt.Format(http.TimeFormat),
		UpdatedAt: t.UpdatedAt.Format(http.TimeFormat),
		Revision:  t.Revision,
	}
}

// ── Federation Trust CRUD ───────────────────────────────────────

// ListTrusts 列出 org 下的 Federation Trusts。
// poolID 留空则返回所有 pools 的 trusts。
func (h *Handler) ListTrusts(ctx context.Context, orgID, poolID string) ([]TrustResponse, int64, error) {
	if strings.TrimSpace(orgID) == "" {
		return nil, 0, apperrors.NewInvalidArgument("org_id is required")
	}
	items, total, err := h.Repo.ListTrusts(ctx, appworkload.ListTrustsQuery{
		OrganizationID: orgID,
		PoolID:         poolID,
		Page:           1,
		PageSize:       50,
	})
	if err != nil {
		return nil, 0, apperrors.WrapInternalf(err, "list federation trusts")
	}
	out := make([]TrustResponse, 0, len(items))
	for _, t := range items {
		out = append(out, trustToResponse(t))
	}
	return out, total, nil
}

// CreateTrust 创建 Federation Trust（admin）。所有安全白名单均由调用方显式提供。
func (h *Handler) CreateTrust(ctx context.Context, orgID, trustID string, req CreateTrustRequest) (TrustResponse, error) {
	if strings.TrimSpace(orgID) == "" {
		return TrustResponse{}, apperrors.NewInvalidArgument("org_id is required")
	}
	if req.PoolID == "" || req.Name == "" || req.Issuer == "" {
		return TrustResponse{}, apperrors.NewInvalidArgument("pool_id, name, and issuer are required")
	}
	if err := appworkload.ValidateHTTPSURI(req.Issuer); err != nil {
		return TrustResponse{}, apperrors.NewInvalidArgument("issuer must be a valid HTTPS URI")
	}
	if err := appworkload.ValidateHTTPSURI(req.JWKSURI); err != nil {
		return TrustResponse{}, apperrors.NewInvalidArgument("jwks_uri must be a valid HTTPS URI")
	}
	if len(req.Audience) == 0 {
		return TrustResponse{}, apperrors.NewInvalidArgument("audience must not be empty")
	}
	for _, audience := range req.Audience {
		if strings.TrimSpace(audience) == "" {
			return TrustResponse{}, apperrors.NewInvalidArgument("audience values must not be empty")
		}
	}
	if err := appworkload.ValidateSubjectPatterns(req.Subjects); err != nil {
		return TrustResponse{}, apperrors.NewInvalidArgument(err.Error())
	}
	pool, err := h.Repo.GetPool(ctx, req.PoolID, orgID)
	if err != nil {
		return TrustResponse{}, err
	}
	if pool == nil {
		return TrustResponse{}, apperrors.NewNotFound("identity pool not found in organization")
	}

	t := &appworkload.FederationTrust{
		ID:             trustID,
		OrganizationID: orgID,
		PoolID:         req.PoolID,
		Name:           req.Name,
		Issuer:         req.Issuer,
		JWKSURI:        req.JWKSURI,
		Audience:       append([]string{}, req.Audience...),
		Subjects:       append([]string{}, req.Subjects...),
		Status:         appworkload.TrustStatusActive,
	}

	if err := h.Repo.CreateTrust(ctx, t); err != nil {
		return TrustResponse{}, apperrors.WrapInternalf(err, "create federation trust")
	}
	return trustToResponse(t), nil
}

// DeleteTrust 软删除 Federation Trust。
func (h *Handler) DeleteTrust(ctx context.Context, orgID, trustID string) error {
	if strings.TrimSpace(orgID) == "" {
		return apperrors.NewInvalidArgument("org_id is required")
	}
	if trustID == "" {
		return apperrors.NewInvalidArgument("trust id is required")
	}
	if err := h.Repo.DeleteTrust(ctx, trustID, orgID); err != nil {
		return apperrors.WrapInternalf(err, "delete federation trust")
	}
	return nil
}

// DisableTrust 软禁用 Federation Trust（不删除，允许重新启用）。
func (h *Handler) DisableTrust(ctx context.Context, orgID, trustID string) error {
	return h.Repo.SetTrustStatus(ctx, trustID, orgID, appworkload.TrustStatusDisabled)
}

// EnableTrust 重新启用已禁用的 Federation Trust。
func (h *Handler) EnableTrust(ctx context.Context, orgID, trustID string) error {
	return h.Repo.SetTrustStatus(ctx, trustID, orgID, appworkload.TrustStatusActive)
}

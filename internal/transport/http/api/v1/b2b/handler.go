package b2b

import (
	"context"

	appb2b "github.com/guliangiam/guliangiam/internal/app/b2b"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
)

// Handler 是 B2B 域名治理 transport 层 —— 将 HTTP 请求转换为
// app/b2b.Service 的方法调用，再将 DTO 序列化为响应。
type Handler struct {
	Service appb2b.Service
}

// ── ClaimDomain ─────────────────────────────────────────────────────────────

func (h Handler) ClaimDomain(ctx context.Context, request ClaimDomainRequest) (*ClaimDomainResponse, error) {
	dto, err := h.Service.ClaimDomain(ctx, appb2b.ClaimDomainCommand{
		OrganizationID: request.OrganizationID,
		Domain:         request.Domain,
	})
	if err != nil {
		return nil, err
	}
	return &ClaimDomainResponse{Domain: toDomainResponse(dto)}, nil
}

// ── ListDomains ────────────────────────────────────────────────────────────

func (h Handler) ListDomains(ctx context.Context, request ListDomainsRequest) (ListDomainsResponse, error) {
	if request.Page < 1 {
		return ListDomainsResponse{}, apperrors.NewInvalidArgument("page must be positive")
	}
	if request.PageSize < 1 || request.PageSize > 100 {
		return ListDomainsResponse{}, apperrors.NewInvalidArgument("pageSize must be between 1 and 100")
	}
	if request.Status != "" && !appb2b.Statuses.Contains(request.Status) {
		return ListDomainsResponse{}, apperrors.NewInvalidArgument("invalid domain status")
	}
	items, total, err := h.Service.ListDomains(ctx, appb2b.ListDomainsQuery{
		Page:           request.Page,
		PageSize:       request.PageSize,
		OrganizationID: request.OrganizationID,
		Status:         request.Status,
	})
	if err != nil {
		return ListDomainsResponse{}, err
	}
	domains := make([]DomainResponse, len(items))
	for i, d := range items {
		domains[i] = toDomainResponse(d)
	}
	return presenter.NewListResponse(domains, request.Page, request.PageSize, total), nil
}

// ── DeleteDomain ───────────────────────────────────────────────────────────

// DeleteDomain 撤销认领；组织归属与删除在同一应用事务中校验。
func (h Handler) DeleteDomain(ctx context.Context, req DeleteDomainRequest) error {
	return h.Service.DeleteDomain(ctx, req.DomainID, req.OrganizationID)
}

// ── VerifyDomain (手动触发) ────────────────────────────────────────────────

// VerifyDomain 手动触发 DNS TXT 验证（绕过 Cron 周期）。
// 应用层校验组织归属，transport 不复制业务规则。
func (h Handler) VerifyDomain(ctx context.Context, req VerifyDomainRequest) (*VerifyDomainResponse, error) {
	out, verified, err := h.Service.VerifyDomainByID(ctx, req.DomainID, req.OrganizationID)
	if err != nil {
		return nil, err
	}
	return &VerifyDomainResponse{
		Domain:   toDomainResponse(out),
		Verified: verified,
	}, nil
}

// ── DTO 转换 ────────────────────────────────────────────────────────────────

// toDomainResponse 将 app/b2b.DomainDTO 转为 transport 层 response struct。
func toDomainResponse(dto appb2b.DomainDTO) DomainResponse {
	verifiedAt := &dto.VerifiedAt
	if dto.VerifiedAt.IsZero() {
		verifiedAt = nil
	}
	return DomainResponse{
		ID:                dto.ID,
		OrganizationID:    dto.OrganizationID,
		Domain:            dto.Domain,
		VerificationToken: dto.VerificationToken,
		Status:            dto.Status,
		VerifiedAt:        verifiedAt,
		CreatedAt:         dto.CreatedAt,
		UpdatedAt:         dto.UpdatedAt,
	}
}

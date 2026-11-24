// Package platformadmins 是平台管理员管理面 API（BE-A-05）。
// 高敏操作：审计字段（createdBy/approvedBy/approverId）一律取可信会话身份，
// 请求体携带的同名字段被 route.DecodeJSON 的 DisallowUnknownFields 直接拒绝。
package platformadmins

import (
	"context"
	"errors"
	"time"

	appplatformadmins "github.com/guliangiam/guliangiam/internal/app/platform_admins"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
)

// PlatformAdminService 是本包对 app 层平台管理员能力的窄接口依赖。
// 返回类型全部是 app 层 DTO（transport 不 import domain）；运行期由
// bootstrap 注入 appplatformadmins.DTOAdapter 包装的具体 Service。
// nil 时所有端点 fail-closed 返回 501（参照 payments.Handler.Refund）。
type PlatformAdminService interface {
	CreatePlatformAdmin(ctx context.Context, cmd appplatformadmins.CreatePlatformAdminCommand) (*appplatformadmins.PlatformAdminDetail, error)
	UpdatePlatformAdmin(ctx context.Context, cmd appplatformadmins.UpdatePlatformAdminCommand) (*appplatformadmins.PlatformAdminDetail, error)
	SetActive(ctx context.Context, cmd appplatformadmins.SetActiveCommand) error
	DeletePlatformAdmin(ctx context.Context, cmd appplatformadmins.DeletePlatformAdminCommand) error
	ListPlatformAdmins(ctx context.Context, q appplatformadmins.ListPlatformAdminsQuery) ([]appplatformadmins.PlatformAdminDetail, int64, error)
	GetPlatformAdmin(ctx context.Context, id string) (*appplatformadmins.PlatformAdminDetail, error)
	GetPlatformAdminByUserID(ctx context.Context, userID string) (*appplatformadmins.PlatformAdminDetail, error)
	CreateScope(ctx context.Context, cmd appplatformadmins.CreateScopeCommand) (*appplatformadmins.ScopeDetail, error)
	ApproveScope(ctx context.Context, cmd appplatformadmins.ApproveScopeCommand) error
	DeleteScope(ctx context.Context, cmd appplatformadmins.DeleteScopeCommand) error
	ListScopes(ctx context.Context, q appplatformadmins.ListScopesQuery) ([]appplatformadmins.ScopeDetail, error)
	GetScope(ctx context.Context, scopeID string) (*appplatformadmins.ScopeDetail, error)
}

// Handler 平台管理员管理面 handler。
type Handler struct {
	Service PlatformAdminService
}

// NewHandler 构造 Handler。
func NewHandler(s PlatformAdminService) Handler {
	return Handler{Service: s}
}

func (h Handler) wired() error {
	if h.Service == nil {
		return apperrors.NewUnimplemented("platform admin management is not wired")
	}
	return nil
}

// ─── 可信操作者 ─────────────────────────────────────────────────────────────

// operatorUserID 返回可信会话的 users.id；无会话即 401，绝不信任 body。
func operatorUserID(ctx context.Context) (string, error) {
	session, ok := authtransport.SessionContextFromContext(ctx)
	if !ok || session.UserID == "" {
		return "", apperrors.NewUnauthorized("authentication required")
	}
	return session.UserID, nil
}

// operatorAdminID 把可信会话用户解析为其 platform_admins.id，用作管理面写
// 操作的资格门（fail-closed 403）。审计字段 created_by/approved_by 的口径是
// users.id（FK→users.id），调用方应另取 operatorUserID。
func (h Handler) operatorAdminID(ctx context.Context) (string, error) {
	userID, err := operatorUserID(ctx)
	if err != nil {
		return "", err
	}
	admin, err := h.Service.GetPlatformAdminByUserID(ctx, userID)
	if err != nil {
		var appErr *apperrors.Error
		if errors.As(err, &appErr) && appErr.Code == apperrors.CodeNotFound {
			return "", apperrors.NewForbidden("operator is not a registered platform admin")
		}
		return "", err
	}
	if admin == nil || !admin.IsActive {
		return "", apperrors.NewForbidden("operator is not an active platform admin")
	}
	return admin.ID, nil
}

func validateID(id string) error {
	if !appplatformadmins.IsValidUUID(id) {
		return apperrors.NewInvalidArgument("id must be a UUID")
	}
	return nil
}

// ─── 请求体 ─────────────────────────────────────────────────────────────────
// 刻意不声明 createdBy/approvedBy/approverId 字段：DecodeJSON 的
// DisallowUnknownFields 会把携带这些字段的请求体拒为 400。

// CreatePlatformAdminRequest 是 POST /platform-admins 的请求体。
type CreatePlatformAdminRequest struct {
	UserID string `json:"userId"`
}

// PatchPlatformAdminRequest 是 PATCH /platform-admins/{id} 的请求体。
// 当前唯一可写审计字段 created_by 由可信操作者覆盖，请求体必须为空对象。
type PatchPlatformAdminRequest struct{}

// SetActiveRequest 是 POST /platform-admins/{id}:set-active 的请求体。
type SetActiveRequest struct {
	Active *bool `json:"active"`
}

// CreateScopeRequest 是 POST /platform-admins/{id}/scopes 的请求体。
// 审批人固定为可信操作者（app 层禁止创建未审批 scope，也禁止自审批）。
type CreateScopeRequest struct {
	ScopeType     string    `json:"scopeType"`
	TargetTenants []string  `json:"targetTenants"`
	ExpiresAt     time.Time `json:"expiresAt"`
}

// ApproveScopeRequest 是 POST /platform-admin-scopes/{scopeId}:approve 的请求体。
// 审批者固定为可信操作者，请求体必须为空对象。
type ApproveScopeRequest struct{}

// ─── 用例 ───────────────────────────────────────────────────────────────────

// List 分页查询平台管理员。
func (h Handler) List(ctx context.Context, q appplatformadmins.ListPlatformAdminsQuery) (ListPlatformAdminsResponse, error) {
	if err := h.wired(); err != nil {
		return ListPlatformAdminsResponse{}, err
	}
	items, total, err := h.Service.ListPlatformAdmins(ctx, q)
	if err != nil {
		return ListPlatformAdminsResponse{}, err
	}
	return presenter.NewListResponse(items, q.Page, q.PageSize, total), nil
}

// Get 按 ID 查询平台管理员。
func (h Handler) Get(ctx context.Context, id string) (*PlatformAdminResponse, error) {
	if err := h.wired(); err != nil {
		return nil, err
	}
	if err := validateID(id); err != nil {
		return nil, err
	}
	return h.Service.GetPlatformAdmin(ctx, id)
}

// Create 注册平台管理员；created_by 取可信操作者的 users.id（FK→users.id）。
func (h Handler) Create(ctx context.Context, request CreatePlatformAdminRequest) (*PlatformAdminResponse, error) {
	if err := h.wired(); err != nil {
		return nil, err
	}
	if request.UserID == "" {
		return nil, apperrors.NewInvalidArgument("userId is required")
	}
	if !appplatformadmins.IsValidUUID(request.UserID) {
		return nil, apperrors.NewInvalidArgument("userId must be a UUID")
	}
	userID, err := operatorUserID(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := h.operatorAdminID(ctx); err != nil {
		return nil, err
	}
	return h.Service.CreatePlatformAdmin(ctx, appplatformadmins.CreatePlatformAdminCommand{
		UserID:    request.UserID,
		CreatedBy: userID,
	})
}

// Patch 更新审计字段：created_by 覆盖为可信操作者（最后经手人），
// 请求体声称的任何字段都已被 DecodeJSON 拒绝。
func (h Handler) Patch(ctx context.Context, id string, _ PatchPlatformAdminRequest) (*PlatformAdminResponse, error) {
	if err := h.wired(); err != nil {
		return nil, err
	}
	if err := validateID(id); err != nil {
		return nil, err
	}
	userID, err := operatorUserID(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := h.operatorAdminID(ctx); err != nil {
		return nil, err
	}
	return h.Service.UpdatePlatformAdmin(ctx, appplatformadmins.UpdatePlatformAdminCommand{
		ID:        id,
		CreatedBy: userID,
	})
}

// SetActive 启用/停用平台管理员；停用即时吊销会话由 app 层保证。
func (h Handler) SetActive(ctx context.Context, id string, active *bool) error {
	if err := h.wired(); err != nil {
		return err
	}
	if err := validateID(id); err != nil {
		return err
	}
	if active == nil {
		return apperrors.NewInvalidArgument("active is required")
	}
	return h.Service.SetActive(ctx, appplatformadmins.SetActiveCommand{ID: id, Active: *active})
}

// Delete 硬删除平台管理员（不可逆；app 层同事务吊销会话）。
func (h Handler) Delete(ctx context.Context, id string) error {
	if err := h.wired(); err != nil {
		return err
	}
	if err := validateID(id); err != nil {
		return err
	}
	return h.Service.DeletePlatformAdmin(ctx, appplatformadmins.DeletePlatformAdminCommand{ID: id})
}

// ListScopes 列出指定平台管理员的作用域。
func (h Handler) ListScopes(ctx context.Context, platformAdminID string, onlyActive bool) (ListScopesResponse, error) {
	if err := h.wired(); err != nil {
		return ListScopesResponse{}, err
	}
	if err := validateID(platformAdminID); err != nil {
		return ListScopesResponse{}, err
	}
	items, err := h.Service.ListScopes(ctx, appplatformadmins.ListScopesQuery{
		PlatformAdminID: platformAdminID,
		OnlyActive:      onlyActive,
	})
	if err != nil {
		return ListScopesResponse{}, err
	}
	return presenter.NewListResponse(items, 1, len(items), int64(len(items))), nil
}

// CreateScope 为平台管理员创建作用域。approved_by 固定为可信操作者的 users.id
// （Schema FK → users.id；app 层拒绝未审批创建与自审批），created_by 同为操作者 users.id。
func (h Handler) CreateScope(ctx context.Context, platformAdminID string, request CreateScopeRequest) (*ScopeResponse, error) {
	if err := h.wired(); err != nil {
		return nil, err
	}
	if err := validateID(platformAdminID); err != nil {
		return nil, err
	}
	userID, err := operatorUserID(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := h.operatorAdminID(ctx); err != nil {
		return nil, err
	}
	return h.Service.CreateScope(ctx, appplatformadmins.CreateScopeCommand{
		PlatformAdminID: platformAdminID,
		ScopeType:       request.ScopeType,
		TargetTenants:   request.TargetTenants,
		ApprovedBy:      &userID,
		ExpiresAt:       request.ExpiresAt,
		CreatedBy:       userID,
	})
}

// GetScope 按 ID 查询作用域。
func (h Handler) GetScope(ctx context.Context, scopeID string) (*ScopeResponse, error) {
	if err := h.wired(); err != nil {
		return nil, err
	}
	if err := validateID(scopeID); err != nil {
		return nil, err
	}
	return h.Service.GetScope(ctx, scopeID)
}

// ApproveScope 审批作用域；审批者固定为可信操作者的 users.id（FK approved_by → users.id）。
func (h Handler) ApproveScope(ctx context.Context, scopeID string, _ ApproveScopeRequest) error {
	if err := h.wired(); err != nil {
		return err
	}
	if err := validateID(scopeID); err != nil {
		return err
	}
	userID, err := operatorUserID(ctx)
	if err != nil {
		return err
	}
	if _, err := h.operatorAdminID(ctx); err != nil {
		return err
	}
	return h.Service.ApproveScope(ctx, appplatformadmins.ApproveScopeCommand{
		ScopeID:    scopeID,
		ApproverID: userID,
	})
}

// DeleteScope 删除作用域。
func (h Handler) DeleteScope(ctx context.Context, scopeID string) error {
	if err := h.wired(); err != nil {
		return err
	}
	if err := validateID(scopeID); err != nil {
		return err
	}
	return h.Service.DeleteScope(ctx, appplatformadmins.DeleteScopeCommand{ScopeID: scopeID})
}

package platform_admins

import (
	"context"
)

// DTOAdapter 是 Service 的薄包装：方法签名与 Service 一致，但返回 app 层 DTO
// 而不是 domain 实体，使 transport 的窄接口（platformadmins.PlatformAdminService）
// 可以在不 import domain 的前提下被满足（BE-A-05，Clean Architecture Rule 2）。
// 业务规则、事务边界与审计仍由内层 Service 负责，适配器只做投影。
type DTOAdapter struct {
	Service Service
}

// NewDTOAdapter 构造 DTO 投影适配器。
func NewDTOAdapter(s Service) DTOAdapter {
	return DTOAdapter{Service: s}
}

func (a DTOAdapter) CreatePlatformAdmin(ctx context.Context, cmd CreatePlatformAdminCommand) (*PlatformAdminDetail, error) {
	pa, err := a.Service.CreatePlatformAdmin(ctx, cmd)
	if err != nil {
		return nil, err
	}
	return NewPlatformAdminDetail(pa), nil
}

func (a DTOAdapter) UpdatePlatformAdmin(ctx context.Context, cmd UpdatePlatformAdminCommand) (*PlatformAdminDetail, error) {
	pa, err := a.Service.UpdatePlatformAdmin(ctx, cmd)
	if err != nil {
		return nil, err
	}
	return NewPlatformAdminDetail(pa), nil
}

func (a DTOAdapter) SetActive(ctx context.Context, cmd SetActiveCommand) error {
	return a.Service.SetActive(ctx, cmd)
}

func (a DTOAdapter) DeletePlatformAdmin(ctx context.Context, cmd DeletePlatformAdminCommand) error {
	return a.Service.DeletePlatformAdmin(ctx, cmd)
}

func (a DTOAdapter) ListPlatformAdmins(ctx context.Context, q ListPlatformAdminsQuery) ([]PlatformAdminDetail, int64, error) {
	items, total, err := a.Service.ListPlatformAdmins(ctx, q)
	if err != nil {
		return nil, 0, err
	}
	return NewPlatformAdminDetails(items), total, nil
}

func (a DTOAdapter) GetPlatformAdmin(ctx context.Context, id string) (*PlatformAdminDetail, error) {
	pa, err := a.Service.GetPlatformAdmin(ctx, id)
	if err != nil {
		return nil, err
	}
	return NewPlatformAdminDetail(pa), nil
}

func (a DTOAdapter) GetPlatformAdminByUserID(ctx context.Context, userID string) (*PlatformAdminDetail, error) {
	pa, err := a.Service.GetPlatformAdminByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return NewPlatformAdminDetail(pa), nil
}

func (a DTOAdapter) CreateScope(ctx context.Context, cmd CreateScopeCommand) (*ScopeDetail, error) {
	scope, err := a.Service.CreateScope(ctx, cmd)
	if err != nil {
		return nil, err
	}
	return NewScopeDetail(scope), nil
}

func (a DTOAdapter) ApproveScope(ctx context.Context, cmd ApproveScopeCommand) error {
	return a.Service.ApproveScope(ctx, cmd)
}

func (a DTOAdapter) DeleteScope(ctx context.Context, cmd DeleteScopeCommand) error {
	return a.Service.DeleteScope(ctx, cmd)
}

func (a DTOAdapter) ListScopes(ctx context.Context, q ListScopesQuery) ([]ScopeDetail, error) {
	items, err := a.Service.ListScopes(ctx, q)
	if err != nil {
		return nil, err
	}
	return NewScopeDetails(items), nil
}

func (a DTOAdapter) GetScope(ctx context.Context, scopeID string) (*ScopeDetail, error) {
	scope, err := a.Service.GetScope(ctx, scopeID)
	if err != nil {
		return nil, err
	}
	return NewScopeDetail(scope), nil
}

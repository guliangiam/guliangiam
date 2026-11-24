package platform_admins

import (
	platformadmins "github.com/guliangiam/guliangiam/internal/domain/platform_admins"
)

// NewPlatformAdminDetail 把 domain 实体投影为读侧 DTO；nil 入参返回 nil，
// 让 NotFound 等错误路径不需要先判空再映射。
func NewPlatformAdminDetail(pa *platformadmins.PlatformAdmin) *PlatformAdminDetail {
	if pa == nil {
		return nil
	}
	return &PlatformAdminDetail{
		ID:        pa.ID,
		UserID:    pa.UserID,
		CreatedBy: pa.CreatedBy,
		IsActive:  pa.IsActive,
		CreatedAt: pa.CreatedAt,
		UpdatedAt: pa.UpdatedAt,
		Revision:  pa.Revision,
	}
}

// NewPlatformAdminDetails 批量投影；nil 切片映射为空切片，保证列表契约
// 序列化出 [] 而不是 null（presenter.NewListResponse 同口径）。
func NewPlatformAdminDetails(items []*platformadmins.PlatformAdmin) []PlatformAdminDetail {
	details := make([]PlatformAdminDetail, 0, len(items))
	for _, item := range items {
		if detail := NewPlatformAdminDetail(item); detail != nil {
			details = append(details, *detail)
		}
	}
	return details
}

// NewScopeDetail 把 scope domain 实体投影为读侧 DTO；nil 入参返回 nil。
func NewScopeDetail(scope *platformadmins.PlatformAdminScope) *ScopeDetail {
	if scope == nil {
		return nil
	}
	targets := scope.TargetTenants
	if targets == nil {
		targets = []string{}
	}
	return &ScopeDetail{
		ID:              scope.ID,
		PlatformAdminID: scope.PlatformAdminID,
		ScopeType:       scope.ScopeType,
		TargetTenants:   targets,
		ApprovedBy:      scope.ApprovedBy,
		ApprovedAt:      scope.ApprovedAt,
		IsBootstrap:     scope.IsBootstrap,
		ExpiresAt:       scope.ExpiresAt,
		CreatedAt:       scope.CreatedAt,
	}
}

// NewScopeDetails 批量投影；nil 切片映射为空切片。
func NewScopeDetails(items []*platformadmins.PlatformAdminScope) []ScopeDetail {
	details := make([]ScopeDetail, 0, len(items))
	for _, item := range items {
		if detail := NewScopeDetail(item); detail != nil {
			details = append(details, *detail)
		}
	}
	return details
}

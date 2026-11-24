package groups

import (
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// 组类型闭集（G7-s 用户裁定）：virtual/physical/department 是唯一合法取值，
// 与前端创建/编辑/筛选选项一致；创建留空回落 virtual，更新不接受空串。
const (
	GroupTypeVirtual    = "virtual"
	GroupTypePhysical   = "physical"
	GroupTypeDepartment = "department"
)

func IsValidGroupType(value string) bool {
	switch value {
	case GroupTypeVirtual, GroupTypePhysical, GroupTypeDepartment:
		return true
	default:
		return false
	}
}

type Group struct {
	ID             string
	OrganizationID string
	Name           string
	DisplayName    string
	Manager        string
	ContactEmail   string
	Type           string
	ParentID       string
	IsTopGroup     bool
	IsEnabled      bool
	Homepage       string
	Users          []string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Revision       int64
}

// Validate checks the group fields for business rule violations.
func (g *Group) Validate() error {
	if g.ID == "" {
		return apperrors.NewInvalidArgument("id is required")
	}
	if g.OrganizationID == "" {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	if g.Name == "" {
		return apperrors.NewInvalidArgument("name is required")
	}
	return nil
}

// HasUser returns true if the given user is a member of the group.
func (g *Group) HasUser(userID string) bool {
	for _, u := range g.Users {
		if u == userID {
			return true
		}
	}
	return false
}

// IsSubGroup returns true if the group has a parent (not a top-level group).
func (g *Group) IsSubGroup() bool {
	return g.ParentID != ""
}

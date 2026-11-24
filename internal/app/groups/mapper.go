package groups

import domain "github.com/guliangiam/guliangiam/internal/domain/groups"

func toListItem(group *domain.Group) GroupListItem {
	return GroupListItem{
		ID:             group.ID,
		OrganizationID: group.OrganizationID,
		Name:           group.Name,
		DisplayName:    group.DisplayName,
		Type:           group.Type,
		ParentID:       group.ParentID,
		IsTopGroup:     group.IsTopGroup,
		IsEnabled:      group.IsEnabled,
		CreatedAt:      group.CreatedAt,
		UpdatedAt:      group.UpdatedAt,
	}
}

func toDetail(group *domain.Group) *GroupDetail {
	return &GroupDetail{
		GroupListItem: toListItem(group),
		Manager:       group.Manager,
		ContactEmail:  group.ContactEmail,
		Homepage:      group.Homepage,
		Users:         append([]string(nil), group.Users...),
		Revision:      group.Revision,
	}
}

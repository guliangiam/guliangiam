package groups

import (
	domain "github.com/guliangiam/guliangiam/internal/domain/groups"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
)

func toGroup(row groupRow) (*domain.Group, error) {
	group := &domain.Group{
		ID:             row.ID,
		OrganizationID: row.OrganizationID,
		Name:           row.Name,
		DisplayName:    row.DisplayName,
		Manager:        row.Manager,
		ContactEmail:   row.ContactEmail,
		Type:           row.Type,
		ParentID:       row.ParentID,
		IsTopGroup:     row.IsTopGroup,
		IsEnabled:      row.IsEnabled,
		Homepage:       row.Homepage,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
		Revision:       row.Revision,
	}
	if err := shared.UnmarshalJSON(row.Users, &group.Users); err != nil {
		return nil, err
	}
	if group.Users == nil {
		group.Users = []string{}
	}
	return group, nil
}

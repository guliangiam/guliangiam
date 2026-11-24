package users

import (
	"context"

	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
)

type ListUsersQuery struct {
	Page           int
	PageSize       int
	OrganizationID string
	Keyword        string
	Status         string
	UserType       string
	SortField      string
	SortOrder      string
	SearchField    string
	SearchValue    string
}

func (s *Service) ListUsers(ctx context.Context, query ListUsersQuery) ([]UserListItem, int64, error) {
	users, total, err := s.UOW.Users().List(ctx, userdomain.ListQuery{
		Page:           query.Page,
		PageSize:       query.PageSize,
		OrganizationID: query.OrganizationID,
		Keyword:        query.Keyword,
		Status:         query.Status,
		UserType:       query.UserType,
		SortField:      query.SortField,
		SortOrder:      query.SortOrder,
		SearchField:    query.SearchField,
		SearchValue:    query.SearchValue,
		// toListItem 只消费主行 + profile；跳过凭据/身份/MFA 聚合加载。
		Lean: true,
	})
	if err != nil {
		return nil, 0, err
	}

	items := make([]UserListItem, 0, len(users))
	for _, user := range users {
		items = append(items, toListItem(user))
	}

	return items, total, nil
}

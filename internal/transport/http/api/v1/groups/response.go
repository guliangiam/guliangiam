package groups

import (
	appgroups "github.com/guliangiam/guliangiam/internal/app/groups"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
)

type ListGroupsResponse = presenter.ListResponse[appgroups.GroupListItem]
type GroupResponse = appgroups.GroupDetail

type CreateGroupResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

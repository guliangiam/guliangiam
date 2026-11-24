package roles

import (
	approles "github.com/guliangiam/guliangiam/internal/app/roles"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
)

type ListRolesResponse = presenter.ListResponse[approles.RoleListItem]
type RoleResponse = approles.RoleDetail

package permissions

import (
	apppermissions "github.com/guliangiam/guliangiam/internal/app/permissions"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
)

type ListPermissionsResponse = presenter.ListResponse[apppermissions.PermissionListItem]
type PermissionResponse = apppermissions.PermissionDetail

package radiusconfigs

import (
	appradiusconfigs "github.com/guliangiam/guliangiam/internal/app/radiusconfigs"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
)

type ListRadiusConfigsResponse = presenter.ListResponse[appradiusconfigs.RadiusConfigListItem]
type RadiusConfigResponse = appradiusconfigs.RadiusConfigDetail

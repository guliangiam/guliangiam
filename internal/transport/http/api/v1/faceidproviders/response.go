package faceidproviders

import (
	appfaceidproviders "github.com/guliangiam/guliangiam/internal/app/faceidproviders"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
)

type ListFaceIdProvidersResponse = presenter.ListResponse[appfaceidproviders.FaceIdProviderListItem]
type FaceIdProviderResponse = appfaceidproviders.FaceIdProviderDetail

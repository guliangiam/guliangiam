package resources

import (
	appresources "github.com/guliangiam/guliangiam/internal/app/resources"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
)

type ListResourcesResponse = presenter.ListResponse[appresources.ResourceListItem]
type ResourceResponse = appresources.ResourceDetail

package captchaproviders

import (
	appcaptchaproviders "github.com/guliangiam/guliangiam/internal/app/captchaproviders"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
)

type ListCaptchaProvidersResponse = presenter.ListResponse[appcaptchaproviders.CaptchaProviderListItem]
type CaptchaProviderResponse = appcaptchaproviders.CaptchaProviderDetail

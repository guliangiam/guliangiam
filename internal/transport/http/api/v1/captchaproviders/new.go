package captchaproviders

import appcaptchaproviders "github.com/guliangiam/guliangiam/internal/app/captchaproviders"

func NewHandler(service appcaptchaproviders.Service) Handler {
	return Handler{Service: service}
}

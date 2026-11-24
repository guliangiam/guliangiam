package tokens

import apptokens "github.com/guliangiam/guliangiam/internal/app/tokens"

func NewHandler(service apptokens.Service) Handler {
	return Handler{Service: service}
}

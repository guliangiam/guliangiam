package invitations

import (
	appinvitations "github.com/guliangiam/guliangiam/internal/app/invitations"
)

func NewHandler(service appinvitations.Service) Handler {
	return Handler{
		Service: service,
	}
}

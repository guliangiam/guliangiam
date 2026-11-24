package users

import (
	"github.com/guliangiam/guliangiam/internal/app/uow"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

func NewService(clk clock.Clock, uuidGenerator platformuuid.Generator, unitOfWork uow.IdentityUoW, scopeAuthorizer ScopeAuthorizer) Service {
	return Service{
		Clock:           clk,
		UUID:            uuidGenerator,
		UOW:             unitOfWork,
		ScopeAuthorizer: scopeAuthorizer,
	}
}

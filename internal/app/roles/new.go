package roles

import (
	"github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/roles"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

func NewService(clockService clock.Clock, uuidGenerator platformuuid.Generator, unitOfWork uow.IdentityUoW, casbinManager domain.PolicyManager) Service {
	return Service{
		Clock:         clockService,
		UUID:          uuidGenerator,
		UOW:           unitOfWork,
		CasbinManager: casbinManager,
	}
}

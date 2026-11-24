package groups

import (
	"github.com/guliangiam/guliangiam/internal/app/uow"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

func NewService(clockService clock.Clock, uuidGenerator platformuuid.Generator, unitOfWork uow.IdentityUoW) Service {
	return Service{
		Clock: clockService,
		UUID:  uuidGenerator,
		UOW:   unitOfWork,
	}
}

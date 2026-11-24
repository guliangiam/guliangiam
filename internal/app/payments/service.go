package payments

import (
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

type Service struct {
	Clock clock.Clock
	UUID  platformuuid.Generator
	UOW   appuow.CommerceUoW
}

func NewService(clk clock.Clock, uuidGenerator platformuuid.Generator, unitOfWork appuow.UnitOfWork) Service {
	return Service{Clock: clk, UUID: uuidGenerator, UOW: unitOfWork}
}

package gateway

import (
	"github.com/guliangiam/guliangiam/internal/app/uow"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

func NewService(clk clock.Clock, uuidGenerator platformuuid.Generator, unitOfWork uow.UnitOfWork) Service {
	return Service{
		Clock: clk,
		UUID:  uuidGenerator,
		UOW:   unitOfWork,
	}
}

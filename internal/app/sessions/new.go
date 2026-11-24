package sessions

import (
	"github.com/guliangiam/guliangiam/internal/app/uow"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
)

func NewService(clockProvider clock.Clock, unitOfWork uow.AuthUoW) Service {
	return Service{Clock: clockProvider, UOW: unitOfWork}
}

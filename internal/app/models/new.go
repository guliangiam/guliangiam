package models

import (
	"github.com/guliangiam/guliangiam/internal/app/uow"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

type Service struct {
	Clock clock.Clock
	UUID  platformuuid.Generator
	UOW   uow.InfraUoW
}

func NewService(clk clock.Clock, uuidGenerator platformuuid.Generator, unitOfWork uow.InfraUoW) Service {
	return Service{Clock: clk, UUID: uuidGenerator, UOW: unitOfWork}
}

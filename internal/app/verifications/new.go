package verifications

import (
	"github.com/guliangiam/guliangiam/internal/app/uow"
	domainverifications "github.com/guliangiam/guliangiam/internal/domain/verifications"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

func NewService(clk clock.Clock, uuidGenerator platformuuid.Generator, unitOfWork uow.AuthUoW) Service {
	return Service{
		Clock:       clk,
		UUID:        uuidGenerator,
		UOW:         unitOfWork,
		RateLimiter: NewRateLimiter(),
	}
}

func NewServiceWithMessaging(clk clock.Clock, uuidGenerator platformuuid.Generator, unitOfWork uow.AuthUoW, resolver domainverifications.SenderResolver) Service {
	return Service{
		Clock:          clk,
		UUID:           uuidGenerator,
		UOW:            unitOfWork,
		SenderResolver: resolver,
		RateLimiter:    NewRateLimiter(),
	}
}

package permissions

import (
	"github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/permissions"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

func NewService(clk clock.Clock, uuidGenerator platformuuid.Generator, unitOfWork uow.UnitOfWork, casbinManager domain.CasbinPolicyManager) Service {
	return Service{
		Clock:         clk,
		UUID:          uuidGenerator,
		UOW:           unitOfWork,
		CasbinManager: casbinManager,
	}
}

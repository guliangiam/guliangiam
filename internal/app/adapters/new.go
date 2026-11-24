package adapters

import (
	"github.com/guliangiam/guliangiam/internal/app/uow"
	domainpermissions "github.com/guliangiam/guliangiam/internal/domain/permissions"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

type Service struct {
	Clock         clock.Clock
	UUID          platformuuid.Generator
	UOW           uow.InfraUoW
	CasbinManager domainpermissions.CasbinPolicyManager
	// ConnectionTester is the infra-backed database probe port. Nil keeps the
	// service constructible; TestConnection then reports an explicit
	// "not configured" error. Wired at the composition root.
	ConnectionTester ConnectionTester
}

func NewService(clk clock.Clock, uuidGenerator platformuuid.Generator, unitOfWork uow.InfraUoW, casbinManager domainpermissions.CasbinPolicyManager) Service {
	return Service{Clock: clk, UUID: uuidGenerator, UOW: unitOfWork, CasbinManager: casbinManager}
}

package resources

import (
	"github.com/guliangiam/guliangiam/internal/app/uow"
	domainresources "github.com/guliangiam/guliangiam/internal/domain/resources"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

func NewService(clk clock.Clock, uuidGenerator platformuuid.Generator, unitOfWork uow.UnitOfWork, storageProvider domainresources.StorageProvider, storageResolver domainresources.StorageProviderResolver) Service {
	return Service{
		Clock:           clk,
		UUID:            uuidGenerator,
		UOW:             unitOfWork,
		Storage:         storageProvider,
		StorageResolver: storageResolver,
	}
}

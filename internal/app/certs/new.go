package certs

import (
	"github.com/guliangiam/guliangiam/internal/app/uow"
	domaincerts "github.com/guliangiam/guliangiam/internal/domain/certs"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

func NewService(clk clock.Clock, uuidGenerator platformuuid.Generator, unitOfWork uow.AuthUoW) Service {
	return Service{
		Clock: clk,
		UUID:  uuidGenerator,
		UOW:   unitOfWork,
	}
}

func NewServiceWithACME(
	clk clock.Clock,
	uuidGenerator platformuuid.Generator,
	unitOfWork uow.AuthUoW,
	acmeRenewer domaincerts.CertificateRenewer,
) Service {
	return Service{
		Clock:       clk,
		UUID:        uuidGenerator,
		UOW:         unitOfWork,
		ACMERenewer: acmeRenewer,
	}
}

package invitations

import (
	"github.com/guliangiam/guliangiam/internal/app/uow"
	domainverifications "github.com/guliangiam/guliangiam/internal/domain/verifications"
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

// NewServiceWithSenderResolver wires real email delivery into Send/Resend.
// The resolver is the same organization-scoped port the verifications module
// uses (appproviders.MessagingResolver implements it), so no new delivery
// architecture is introduced.
func NewServiceWithSenderResolver(clockService clock.Clock, uuidGenerator platformuuid.Generator, unitOfWork uow.IdentityUoW, resolver domainverifications.SenderResolver) Service {
	svc := NewService(clockService, uuidGenerator, unitOfWork)
	svc.SenderResolver = resolver
	return svc
}

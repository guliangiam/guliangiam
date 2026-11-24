package mock

import (
	"testing"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
)

// TestMockImplementsInterface verifies the generated mock satisfies the UnitOfWork interface.
func TestMockImplementsInterface(t *testing.T) {
	var _ appuow.UnitOfWork = NewMockUnitOfWork(nil)
	var _ appuow.IdentityUoW = NewMockIdentityUoW(nil)
	var _ appuow.AuthUoW = NewMockAuthUoW(nil)
	var _ appuow.AuthnUoW = NewMockAuthnUoW(nil)
	var _ appuow.AuditUoW = NewMockAuditUoW(nil)
	var _ appuow.CommerceUoW = NewMockCommerceUoW(nil)
	var _ appuow.InfraUoW = NewMockInfraUoW(nil)
}

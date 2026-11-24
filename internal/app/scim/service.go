package scim

import (
	"github.com/guliangiam/guliangiam/internal/app/jml"
	"github.com/guliangiam/guliangiam/internal/app/uow"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

type Service struct {
	Clock clock.Clock
	UUID  platformuuid.Generator
	UOW   uow.AuthUoW
	// JML wires the ADR-030 Joiner/Leaver lifecycle into the same transaction
	// as the SCIM user write. Nil keeps the legacy behaviour (no lifecycle
	// outbox events, no role binding cleanup on deactivation).
	JML jml.LifecyclePort
}

func NewService(clk clock.Clock, uuidGenerator platformuuid.Generator, unitOfWork uow.AuthUoW) Service {
	return Service{
		Clock: clk,
		UUID:  uuidGenerator,
		UOW:   unitOfWork,
	}
}

// ─────────────────────────────────────────────
// User CRUD
// ─────────────────────────────────────────────

// ListUsers returns a SCIM list of users for the given organization.

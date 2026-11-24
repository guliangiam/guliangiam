package users

import (
	"context"

	"github.com/guliangiam/guliangiam/internal/app/jml"
	"github.com/guliangiam/guliangiam/internal/app/uow"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	scopeauth "github.com/guliangiam/guliangiam/internal/platform/scopeauthorizer"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

// ScopeAuthorizer abstracts cross-tenant scope + ticket authorization.
// Set by runtime injection for platform_admin RiskCritical operations.
type ScopeAuthorizer interface {
	Authorize(ctx context.Context, req scopeauth.AuthorizationRequest) (scopeauth.AuthorizationResult, error)
}

type Service struct {
	Clock           clock.Clock
	UUID            platformuuid.Generator
	UOW             uow.IdentityUoW
	ScopeAuthorizer ScopeAuthorizer // Phase D-Step 3: impersonate RiskCritical 下沉
	// RolePolicies projects user_role_bindings into Casbin `g` rows. Nil keeps
	// legacy behaviour for callers that do not wire a policy store (tests).
	RolePolicies RolePolicyManager
	// IdentityVerification initiates a real third-party identity check. Nil
	// means no provider is wired and verification is refused rather than
	// self-attested.
	IdentityVerification IdentityVerificationGateway
	// ContactVerification sends and consumes email/phone verification codes.
	// Nil means messaging is not wired; callers get 503 instead of a fake pass.
	ContactVerification ContactVerificationGateway
	// DocumentRecognizer runs OCR on uploaded identity documents. Nil means OCR
	// is not wired; callers get 503 instead of fabricated fields.
	DocumentRecognizer IdentityDocumentRecognizer
	// FaceIDEnrollment runs liveness detection and stores the enrolled face
	// image. Nil means no provider is wired and enrollment is refused.
	FaceIDEnrollment FaceIDEnrollmentGateway
	// JML wires the ADR-030 Joiner/Mover lifecycle into the same transaction
	// as the user write. Nil keeps the legacy behaviour (direct role binding
	// replacement, no lifecycle outbox events).
	JML jml.LifecyclePort
	// Notifier delivers account-security messages (administrator password
	// resets) to the personal notification center. Nil disables fan-out.
	Notifier Notifier
}

type CreateUserCommand struct {
	OrganizationID string
	// ActorID is the operator user id (empty for system actions).
	ActorID             string
	Username            string
	ExternalID          string
	UserType            string
	Status              string
	IsAdmin             bool
	SignupApplicationID string
	RegisterSource      string
}

func (s *Service) NewUser(command CreateUserCommand) (*userdomain.User, error) {
	id, err := s.UUID.NewString()
	if err != nil {
		return nil, err
	}

	now := s.Clock.Now()
	return &userdomain.User{
		ID:                  id,
		OrganizationID:      command.OrganizationID,
		Username:            command.Username,
		ExternalID:          command.ExternalID,
		UserType:            command.UserType,
		Status:              command.Status,
		IsAdmin:             command.IsAdmin,
		SignupApplicationID: command.SignupApplicationID,
		RegisterSource:      command.RegisterSource,
		CreatedAt:           now,
		UpdatedAt:           now,
		Revision:            1,
	}, nil
}

// GetUserCount returns the total number of users, optionally filtered by organization.
func (s *Service) GetUserCount(ctx context.Context, organizationID string) (int64, error) {
	// organizationID 来自 chi 的 query/import 参数（users/count、导入预检），未做 TrustedOrganizationID 收敛；Count 按该 org 过滤（空则统计作用域内全部），跨 org 行数由调用方作用域与 RLS 决定。
	return s.UOW.Users().Count(ctx, organizationID)
}

// 租户作用域说明：本包不做 RLS 提升。users 的 organizationId 来自 query/body；跨组织
// 面集中在 /api/v1/global/users 与 GetUserCount（Count 按传入 org 统计），可见性由
// ScopeAuthorizer 与 RLS 决定，service 不提升作用域。

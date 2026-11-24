package users

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
)

// FaceIDEnrollmentRequest is the provider-neutral enrollment input. The users
// application service owns the request shape so it does not import the faceid
// infrastructure or the resources application package.
type FaceIDEnrollmentRequest struct {
	OrganizationID string
	UserID         string
	ProviderID     string // optional: explicit FaceIdProvider row; empty picks the enabled one
	MimeType       string
	DeviceInfo     string
	Data           []byte
}

// FaceIDEnrollmentGateway runs the liveness check and returns the minted
// enrollment identity. Implemented by a bootstrap adapter over the FaceVerifier
// infrastructure. It no longer stores the image: the application service seals
// the captured bytes into user_face_templates so no plaintext底图 is persisted.
type FaceIDEnrollmentGateway interface {
	EnrollFaceID(ctx context.Context, request FaceIDEnrollmentRequest) (userdomain.FaceId, error)
}

// FaceID enrollment audit event types. All follow the naming convention
// "<aggregate>.<action>.<outcome>" so downstream risk alerting can route
// them without parsing free-form messages. Every rejection path writes
// its audit in its own independent short transaction — pre-check rejections
// (before provider) and transaction-scope rejections (after FOR UPDATE lock
// but before commit) both use recordFaceIDRejectAudit so the audit record
// is not swallowed by the rejection rollback.
const (
	faceIDEventCrossTenantRejected = "user.faceid.cross_tenant_rejected"
	faceIDEventRealNameRequired    = "user.faceid.real_name_required"
	faceIDEventLivenessFailed      = "user.faceid.liveness_failed"
	faceIDEventUserGoneUnderLock   = "user.faceid.user_gone_under_lock"
	faceIDEventLockReadMismatch    = "user.faceid.lock_read_mismatch"
)

// faceIDTxReject is returned by EnrollUserFaceID's mutation transaction when
// a post-LockByID rejection path fires. The wrapped Err is surfaced to the
// caller; the audit metadata is consumed by EnrollUserFaceID after the tx
// has rolled back so the audit record lands in its own short transaction
// (never inside a tx that is about to roll back).
type faceIDTxReject struct {
	EventType string
	OrgID     string
	UserID    string
	Payload   map[string]any
	Err       error // surfaced to the caller
}

func (e *faceIDTxReject) Error() string { return e.Err.Error() }
func (e *faceIDTxReject) Unwrap() error { return e.Err }

// faceTemplateTx is a narrow capability interface: appuow.UnitOfWork deliberately
// does not declare FaceTemplates(), so expanding it (and every fake) is avoided.
// The concrete postgres/mysql UoW satisfies this assertion.
type faceTemplateTx interface {
	FaceTemplates() userdomain.FaceTemplateRepository
}

func faceTemplateRepoFrom(src any) (userdomain.FaceTemplateRepository, error) {
	if p, ok := src.(faceTemplateTx); ok {
		if repo := p.FaceTemplates(); repo != nil {
			return repo, nil
		}
	}
	return nil, apperrors.NewInternal("face template repository unavailable")
}

// EnrollUserFaceID registers the user's FaceID login底图. The user must already
// be real-name verified (已实名前置门槛): an unverified user is rejected before
// the provider call so liveness quota is never spent on an ineligible account.
// Tenant ownership is checked before the provider call and re-checked inside a
// FOR UPDATE transaction so a concurrent revocation/deletion cannot slip through.
// The captured image is sealed into user_face_templates (one active row per
// user, re-enrollment updates in place); the legacy face_ids JSONB keeps only a
// non-sensitive marker so no plaintext image URL is stored.
func (s *Service) EnrollUserFaceID(ctx context.Context, request FaceIDEnrollmentRequest) (userdomain.FaceId, error) {
	if err := requireOrganizationID(request.OrganizationID); err != nil {
		return userdomain.FaceId{}, err
	}
	if strings.TrimSpace(request.UserID) == "" {
		return userdomain.FaceId{}, apperrors.NewInvalidArgument("userId is required")
	}
	if len(request.Data) == 0 {
		return userdomain.FaceId{}, apperrors.NewInvalidArgument("face image is required")
	}

	if s.FaceIDEnrollment == nil {
		return userdomain.FaceId{}, apperrors.NewUnavailable("face id enrollment provider is not configured")
	}

	// Tenant ownership and the real-name gate run before the provider call so a
	// cross-tenant or unverified id never reaches the quota-spending liveness check.
	user, err := s.UOW.Users().GetByIDForOrganization(ctx, request.UserID, request.OrganizationID)
	if err != nil {
		s.recordFaceIDRejectAudit(ctx, request.OrganizationID, request.UserID, faceIDEventCrossTenantRejected, map[string]any{
			"decision": "rejected",
			"reason":   "user_not_in_tenant",
		})
		return userdomain.FaceId{}, err
	}
	if !user.IsVerified {
		s.recordFaceIDRejectAudit(ctx, request.OrganizationID, request.UserID, faceIDEventRealNameRequired, map[string]any{
			"decision": "rejected",
			"reason":   "real_name_verification_required",
		})
		return userdomain.FaceId{}, apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "real-name verification is required before enrolling a face id", map[string]any{metaKeyField: "isVerified"})
	}

	faceID, err := s.FaceIDEnrollment.EnrollFaceID(ctx, request)
	if err != nil {
		s.recordFaceIDRejectAudit(ctx, request.OrganizationID, request.UserID, faceIDEventLivenessFailed, map[string]any{
			"providerId": request.ProviderID,
			"reason":     "provider_liveness_check_failed",
		})
		return userdomain.FaceId{}, err
	}

	err = runInTx(ctx, s.UOW, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// Row-level FOR UPDATE lock serializes concurrent enrollments and guards
		// against the user being deleted/revoked between pre-check and commit.
		if err := tx.Users().LockByID(ctx, request.UserID); err != nil {
			return &faceIDTxReject{
				EventType: faceIDEventUserGoneUnderLock,
				OrgID:     request.OrganizationID,
				UserID:    request.UserID,
				Payload:   map[string]any{"decision": "rejected", "reason": "user_not_found_after_precheck"},
				Err:       err,
			}
		}
		current, err := tx.Users().GetByIDForOrganization(ctx, request.UserID, request.OrganizationID)
		if err != nil {
			return &faceIDTxReject{
				EventType: faceIDEventLockReadMismatch,
				OrgID:     request.OrganizationID,
				UserID:    request.UserID,
				Payload:   map[string]any{"reason": "row_locked_but_not_visible_to_org_scoped_read"},
				Err:       err,
			}
		}
		if !current.IsVerified {
			// TOCTOU: real-name verification was revoked after the pre-check.
			return &faceIDTxReject{
				EventType: faceIDEventRealNameRequired,
				OrgID:     current.OrganizationID,
				UserID:    current.ID,
				Payload:   map[string]any{"decision": "rejected", "reason": "real_name_revoked_under_lock"},
				Err:       apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "real-name verification is required before enrolling a face id", map[string]any{metaKeyField: "isVerified"}),
			}
		}
		repo, err := faceTemplateRepoFrom(tx)
		if err != nil {
			return err
		}
		now := s.Clock.Now()
		template := &userdomain.FaceTemplate{
			ID:             faceID.ID,
			OrganizationID: current.OrganizationID,
			UserID:         current.ID,
			TemplateRef:    base64.StdEncoding.EncodeToString(request.Data),
			Status:         userdomain.FaceTemplateStatusActive,
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		if err := repo.Enroll(ctx, template); err != nil {
			return err
		}
		// Legacy JSONB compat: keep a single non-sensitive marker (no plaintext
		// image URL). The authoritative encrypted底图 lives in user_face_templates.
		collections := userdomain.UserCollections{
			Addresses:         current.Addresses,
			ManagedAccounts:   current.ManagedAccounts,
			FaceIds:           []userdomain.FaceId{{ID: faceID.ID, CreatedAt: faceID.CreatedAt}},
			Cart:              current.Cart,
			MfaItems:          current.MfaItems,
			MfaAccounts:       current.MfaAccounts,
			ApplicationScopes: current.ApplicationScopes,
		}
		if err := tx.Users().UpdateCollectionsForOrganization(ctx, request.UserID, request.OrganizationID, collections); err != nil {
			return err
		}
		return s.embedAuditEvent(
			tx, ctx,
			current.OrganizationID,
			"",
			"user.updated",
			"User",
			current.ID,
			"user",
			"user.faceid.enrolled",
			map[string]any{},
		)
	})
	if err != nil {
		var rej *faceIDTxReject
		if errors.As(err, &rej) {
			s.recordFaceIDRejectAudit(ctx, rej.OrgID, rej.UserID, rej.EventType, rej.Payload)
			return userdomain.FaceId{}, rej.Err
		}
		return userdomain.FaceId{}, err
	}
	return faceID, nil
}

// RevokeUserFaceID revokes the user's active FaceID login底图 (吊销). The user
// must belong to the trusted organization; the encrypted row is marked revoked
// rather than deleted so the audit trail survives. Legacy JSONB markers are
// cleared so no stale face id remains selectable.
func (s *Service) RevokeUserFaceID(ctx context.Context, organizationID, userID string) error {
	if err := requireOrganizationID(organizationID); err != nil {
		return err
	}
	if strings.TrimSpace(userID) == "" {
		return apperrors.NewInvalidArgument("userId is required")
	}
	return runInTx(ctx, s.UOW, func(ctx context.Context, tx appuow.UnitOfWork) error {
		current, err := tx.Users().GetByIDForOrganization(ctx, userID, organizationID)
		if err != nil {
			return err
		}
		repo, err := faceTemplateRepoFrom(tx)
		if err != nil {
			return err
		}
		if err := repo.RevokeByUser(ctx, organizationID, userID, s.Clock.Now()); err != nil {
			return err
		}
		collections := userdomain.UserCollections{
			Addresses:         current.Addresses,
			ManagedAccounts:   current.ManagedAccounts,
			FaceIds:           []userdomain.FaceId{},
			Cart:              current.Cart,
			MfaItems:          current.MfaItems,
			MfaAccounts:       current.MfaAccounts,
			ApplicationScopes: current.ApplicationScopes,
		}
		if err := tx.Users().UpdateCollectionsForOrganization(ctx, userID, organizationID, collections); err != nil {
			return err
		}
		return s.embedAuditEvent(
			tx, ctx,
			organizationID, "",
			"user.updated", "User", userID,
			"user", "user.faceid.revoked",
			map[string]any{},
		)
	})
}

// recordFaceIDRejectAudit writes one audit record + outbox event in its own
// short-lived System() transaction. This is the single call site for all
// FaceID rejection audits (pre-check and transaction-scope alike) — it is
// never invoked inside a mutation transaction because a write inside a tx
// that is about to rollback is useless. Write failures are best-effort:
// the caller's rejection error always surfaces regardless.
func (s *Service) recordFaceIDRejectAudit(
	ctx context.Context, orgID, userID, eventType string, payload map[string]any,
) {
	_ = s.UOW.WithinTransaction(tenantctx.System(ctx), func(ctx context.Context, tx appuow.UnitOfWork) error {
		return s.embedAuditEvent(
			tx, ctx,
			orgID, "",
			"user.faceid.rejected", "User", userID,
			"user", eventType,
			payload,
		)
	})
}
